package cog

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/grafov/m3u8"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jessevdk/go-flags"
	"github.com/ricochet2200/go-disk-usage/du"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/discord"
	httpclient "github.com/xIceArcher/go-leah/http"
	"github.com/xIceArcher/go-leah/qnap"
	"github.com/xIceArcher/go-leah/weibo"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type DownloadCog struct {
	GenericCog

	qnapConfig *config.QNAPConfig
}

func NewDownloadCog(cfg *config.Config, s *discord.Session) (Cog, error) {
	c := &DownloadCog{
		qnapConfig: cfg.QNAP,
	}

	c.allCommands = map[string]CommandFunc{
		"disk":       c.Disk,
		"streamlink": c.Streamlink,
		"weibo":      c.Weibo,
	}

	return c, nil
}

func (c *DownloadCog) Disk(ctx context.Context, s *discord.MessageSession, args []string) {
	usage := du.NewDiskUsage(".")

	s.SendMessage(fmt.Sprintf("Available space: %v", units.HumanSize(float64(usage.Free()))))
}

func (c *DownloadCog) Streamlink(ctx context.Context, s *discord.MessageSession, args []string) {
	type Args struct {
		HTTPHeader  string `short:"h" long:"header"`
		HTTPCookies string `short:"c" long:"cookie"`
		Delete      bool   `short:"d" long:"delete"`

		Args struct {
			M3U8URLStr string `required:"yes"`
			FileName   string `required:"yes"`
		} `positional-args:"yes"`
	}

	commandArgs := &Args{}
	_, err := flags.NewParser(commandArgs, flags.IgnoreUnknown).ParseArgs(args)
	if err != nil {
		s.SendError(err)
		return
	}

	client := httpclient.NewClientWithHeadersAndCookiesStr(commandArgs.HTTPHeader, commandArgs.HTTPCookies)
	resp, err := client.Get(commandArgs.Args.M3U8URLStr)
	if err != nil {
		s.SendError(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.SendErrorf("HTTP request to %s returned status %v", resp.Request.URL.String(), resp.StatusCode)
		return
	}

	genericPlaylist, _, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		s.SendError(err)
		return
	}

	switch playlist := genericPlaylist.(type) {
	case *m3u8.MasterPlaylist:
		var bestVariant *m3u8.Variant
		for _, variant := range playlist.Variants {
			if bestVariant == nil || variant.Bandwidth > bestVariant.Bandwidth {
				bestVariant = variant
			}
		}

		m3u8Url, err := url.Parse(commandArgs.Args.M3U8URLStr)
		if err != nil {
			s.SendError(err)
			return
		}

		mediaUrl, err := m3u8Url.Parse(bestVariant.URI)
		if err != nil {
			s.SendError(err)
			return
		}

		args[len(args)-2] = mediaUrl.String()
		c.Streamlink(ctx, s, args)
	case *m3u8.MediaPlaylist:
		dir := fmt.Sprintf("%s-%s", time.Now().Format(consts.TimeFormatYYMMDDHHMMSS), s.ChannelID)
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			s.SendError(err)
			return
		}

		if c.qnapConfig.IsEnabled {
			exists, err := checkFileExists(ctx, s, c.qnapConfig, commandArgs.Args.FileName)
			if err != nil {
				s.SendError(err)
				return
			}

			if exists {
				s.SendMessage("File %s already exists!", commandArgs.Args.FileName)
				return
			}
		}

		s.SendMessage("Starting to download %s", commandArgs.Args.FileName)

		downloadedRuns, err := handleMediaPlaylist(ctx, s, client, commandArgs.Args.M3U8URLStr, playlist.Key, dir)
		if err != nil {
			s.SendError(err)
			return
		}

		if c.qnapConfig.IsEnabled {
			if _, err := handleUpload(c.qnapConfig, s, commandArgs.Args.FileName, downloadedRuns); err != nil {
				s.SendError(err)
				return
			}

			if commandArgs.Delete {
				s.SendMessage("Clearing disk space...")
				if err := os.RemoveAll(dir); err != nil {
					s.SendError(err)
					return
				}
			}
		}

		c.Disk(ctx, s, []string{})
	}
}

type DownloadedFile struct {
	Name  string
	SeqNo int
}

type PlaylistConfig struct {
	Directory   string
	IsEncrypted bool
	Block       cipher.Block
}

type Segment struct {
	FileName string
	URL      *url.URL
	IV       []byte
}

func checkFileExists(ctx context.Context, s *discord.MessageSession, qnapConfig *config.QNAPConfig, fileNameStr string) (bool, error) {
	extension := path.Ext(fileNameStr)
	fileName := strings.TrimSuffix(fileNameStr, extension)

	if extension == "" {
		extension = ".ts"
	}

	qnapAPI, err := qnap.New(qnapConfig.URL, s.Logger)
	if err != nil {
		return false, err
	}

	if err := qnapAPI.Login(qnapConfig.Username, qnapConfig.Password); err != nil {
		return false, err
	}
	defer qnapAPI.Logout()

	return qnapAPI.Exists(qnapConfig.DownloadBasePath, fmt.Sprintf("%s%s", fileName, extension))
}

func handleMediaPlaylist(ctx context.Context, s *discord.MessageSession, client *retryablehttp.Client, m3u8UrlStr string, key *m3u8.Key, dir string) (downloadRuns [][]string, err error) {
	isEncrypted := key != nil
	currRunNo := 0

	m3u8Url, err := url.Parse(m3u8UrlStr)
	if err != nil {
		return nil, err
	}

	var block cipher.Block
	if isEncrypted {
		keyBytes, err := downloadKey(ctx, client, m3u8Url, key)
		if err != nil {
			return nil, err
		}

		block, err = aes.NewCipher(keyBytes)
		if err != nil {
			return nil, err
		}
	}

	bar, err := s.SendBytesProgressBar(1, "Downloading")
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	toHeadChan := make(chan *Segment, 10000)
	toGetChan := make(chan *Segment, 10000)

	wg.Add(1)
	go func() {
		defer wg.Done()
		getSegmentSize(ctx, client, toHeadChan, toGetChan, bar, s.Logger)
	}()

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadSegment(ctx, &PlaylistConfig{
				Directory:   dir,
				IsEncrypted: isEncrypted,
				Block:       block,
			}, client, toGetChan, bar, s.Logger)
		}()
	}

	defer func() {
		close(toHeadChan)
		wg.Wait()
		bar.Add(1)
	}()

	downloadedRuns := make([]map[int]string, 0)
	downloadedRuns = append(downloadedRuns, make(map[int]string))

	doneCh := make(chan int, 1)

	var sleepTime time.Duration
	errCount := 0

	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("Context cancelled, download aborted")
			return nil, nil
		case <-doneCh:
			s.Logger.Info("Stream closed")
			runs := make([][]*DownloadedFile, 0, len(downloadedRuns))
			for runNo, runSegments := range downloadedRuns {
				runs = append(runs, make([]*DownloadedFile, 0, len(runSegments)))
				for seqNo, filePath := range runSegments {
					runs[runNo] = append(runs[runNo], &DownloadedFile{
						Name:  filePath,
						SeqNo: seqNo,
					})
				}

				slices.SortFunc(runs[runNo], func(i, j *DownloadedFile) bool {
					return i.SeqNo < j.SeqNo
				})
			}

			downloadRuns = make([][]string, 0, len(runs))
			for runNo, run := range runs {
				downloadRuns = append(downloadRuns, make([]string, 0, len(runs)))
				for _, file := range run {
					downloadRuns[runNo] = append(downloadRuns[runNo], file.Name)
				}
			}

			return downloadRuns, nil
		case <-time.After(sleepTime):
			if err := func() error {
				resp, err := client.Get(m3u8Url.String())
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("HTTP request to %s returned status %v", resp.Request.URL.String(), resp.StatusCode)
				}

				playlist, _, err := m3u8.DecodeFrom(resp.Body, true)
				if err != nil {
					return err
				}

				mediaList, ok := playlist.(*m3u8.MediaPlaylist)
				if !ok {
					return fmt.Errorf("not a media playlist")
				}
				sleepTime = time.Duration(mediaList.TargetDuration) * time.Second

				for i, segment := range mediaList.Segments {
					if segment == nil {
						continue
					}

					segmentUrl, err := m3u8Url.Parse(segment.URI)
					if err != nil {
						return err
					}

					fileName := filepath.Join(dir, path.Base(segmentUrl.Path))

					seqNo := i + int(mediaList.SeqNo)
					currRunSegments := downloadedRuns[currRunNo]
					if existingFileName, ok := currRunSegments[seqNo]; ok {
						if fileName == existingFileName {
							continue
						}

						currRunNo++
						downloadedRuns = append(downloadedRuns, make(map[int]string))
					}

					var iv []byte
					if isEncrypted {
						iv = make([]byte, 16)

						if mediaList.Key.IV == "" {
							binary.BigEndian.PutUint16(iv, uint16(i+int(mediaList.SeqNo)))
						} else {
							ivParsed := new(big.Int)
							ivParsed.SetString(mediaList.Key.IV, 16)

							ivParsed.FillBytes(iv)
						}
					}

					toHeadChan <- &Segment{
						FileName: fileName,
						URL:      segmentUrl,
						IV:       iv,
					}

					downloadedRuns[currRunNo][seqNo] = fileName
				}

				if mediaList.Closed {
					doneCh <- 1
				}

				return nil
			}(); err != nil {
				s.Logger.With(zap.Int("errCount", errCount)).Error(err)
				errCount++

				if errCount >= 10 {
					s.Logger.Error("Too many errors when getting M3U8, aborting...")
					doneCh <- 1
				}

				break
			}

			errCount = 0
		}
	}
}

func downloadKey(ctx context.Context, client *retryablehttp.Client, m3u8Url *url.URL, key *m3u8.Key) ([]byte, error) {
	keyUrl, err := m3u8Url.Parse(key.URI)
	if err != nil {
		return nil, err
	}

	resp, err := client.Get(keyUrl.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request to %s returned status %v", resp.Request.URL.String(), resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func getSegmentSize(ctx context.Context, client *retryablehttp.Client, in <-chan *Segment, out chan<- *Segment, bar *discord.ProgressBar, logger *zap.SugaredLogger) {
	for {
		select {
		case <-ctx.Done():
			return
		case segment, ok := <-in:
			if !ok {
				close(out)
				return
			}

			resp, err := client.Head(segment.URL.String())
			if err != nil || resp.StatusCode != http.StatusOK {
				logger.With(zap.Error(err)).Warn("Failed to HEAD segment URL")
			} else {
				bar.AddMax(resp.ContentLength)
			}

			out <- segment
		}
	}
}

func downloadSegment(ctx context.Context, cfg *PlaylistConfig, client *retryablehttp.Client, segmentChan <-chan *Segment, bar *discord.ProgressBar, logger *zap.SugaredLogger) {
	for {
		select {
		case <-ctx.Done():
			return
		case segment, ok := <-segmentChan:
			if !ok {
				return
			}

			segmentLogger := logger.With(zap.String("url", segment.URL.String()))

			numRetries := 0
			for numRetries < 5 {
				if err := func() error {
					out, err := os.Create(segment.FileName)
					if err != nil {
						return err
					}
					defer out.Close()

					resp, err := client.Get(segment.URL.String())
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("HTTP request to %s returned status %v", resp.Request.URL.String(), resp.StatusCode)
					}

					bytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return err
					}
					bar.Add(int64(len(bytes)))

					if cfg.IsEncrypted {
						mode := cipher.NewCBCDecrypter(cfg.Block, segment.IV)
						mode.CryptBlocks(bytes, bytes)
					}

					if _, err = out.Write(bytes); err != nil {
						return err
					}

					segmentLogger.Info("Downloaded")
					return nil
				}(); err != nil {
					segmentLogger.With(zap.Int("numRetries", numRetries)).Error(err)
					numRetries++
					continue
				}

				break
			}
		}
	}
}

func handleUpload(qnapConfig *config.QNAPConfig, s *discord.MessageSession, fileNameStr string, downloadedRuns [][]string) (int64, error) {
	extension := path.Ext(fileNameStr)
	fileName := strings.TrimSuffix(fileNameStr, extension)

	if extension == "" {
		extension = ".ts"
	}

	if len(downloadedRuns) == 1 {
		return uploadAndConcatFiles(qnapConfig, s, fmt.Sprintf("%s%s", fileName, extension), downloadedRuns[0])
	} else {
		var totalFileSize int64
		for runNo, run := range downloadedRuns {
			runFileName := fmt.Sprintf("%s_%v%s", fileName, runNo+1, extension)
			currFileSize, err := uploadAndConcatFiles(qnapConfig, s, runFileName, run)
			if err != nil {
				return totalFileSize, err
			}
			totalFileSize += currFileSize
		}
		return totalFileSize, nil
	}
}

func uploadAndConcatFiles(qnapConfig *config.QNAPConfig, s *discord.MessageSession, fileName string, filePaths []string) (int64, error) {
	bar, err := s.SendBytesProgressBar(1*units.TiB, fmt.Sprintf("Uploading %s", fileName))
	if err != nil {
		msg := "Failed to initialize progress bar"
		s.Logger.With(zap.Error(err)).Warn(msg)
		s.SendError(fmt.Errorf(msg))
	}

	qnapAPI, err := qnap.New(qnapConfig.URL, s.Logger)
	if err != nil {
		return 0, err
	}

	if err := qnapAPI.Login(qnapConfig.Username, qnapConfig.Password); err != nil {
		return 0, err
	}
	defer qnapAPI.Logout()

	return qnapAPI.UploadAndConcat(qnapConfig.DownloadBasePath, fileName, filePaths, bar)
}

func (c *DownloadCog) Weibo(ctx context.Context, s *discord.MessageSession, args []string) {
	links, dirName := args[:len(args)-1], args[len(args)-1]

	postIDs := make([]string, 0, len(links))
	for _, link := range links {
		regex := regexp.MustCompile(`http[s]?://(?:w{3}\.)?weibo.com/[0-9]+/([A-Za-z0-9]+)`)
		matches := regex.FindStringSubmatch(link)
		if len(matches) <= 1 {
			continue
		}
		postIDs = append(postIDs, matches[1])
	}

	weiboAPI := weibo.NewAPI()

	photos := make([]*weibo.PhotoVariant, 0)
	for _, id := range postIDs {
		post, err := weiboAPI.GetPost(id)
		if err != nil {
			s.SendError(err)
			continue
		}

		for _, photo := range post.Photos {
			photos = append(photos, photo.BestVariant())
		}
	}

	bar, err := s.SendBytesProgressBar(0, fmt.Sprintf("Downloading %s", dirName))
	if err != nil {
		s.SendError(err)
		return
	}

	for _, photo := range photos {
		photoSize, err := weiboAPI.GetPhotoVariantSize(photo)
		if err != nil {
			s.SendError(err)
			continue
		}

		bar.AddMax(photoSize)
	}

	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		s.SendError(err)
		return
	}
	defer os.RemoveAll(tempDir)

	filePaths := make([]string, 0, len(photos))

	for i, photo := range photos {
		filePath := filepath.Join(tempDir, fmt.Sprintf("%v.jpg", i+1))

		f, err := os.Create(filePath)
		if err != nil {
			s.SendError(err)
			continue
		}

		photoBytes, err := weiboAPI.DownloadPhotoVariant(photo)
		if err != nil {
			s.SendError(err)
			continue
		}

		if _, err := f.Write(photoBytes); err != nil {
			s.SendError(err)
			continue
		}

		filePaths = append(filePaths, filePath)
		bar.Add(int64(len(photoBytes)))
		f.Close()
	}

	if c.qnapConfig.IsEnabled {
		if err := uploadFiles(c.qnapConfig, s, dirName, filePaths); err != nil {
			s.SendError(err)
		}
	}
}

func uploadFiles(qnapConfig *config.QNAPConfig, s *discord.MessageSession, dirName string, filePaths []string) error {
	bar, err := s.SendBytesProgressBar(1*units.TiB, fmt.Sprintf("Uploading %s", dirName))
	if err != nil {
		msg := "Failed to initialize progress bar"
		s.Logger.With(zap.Error(err)).Warn(msg)
		s.SendError(fmt.Errorf(msg))
	}

	qnapAPI, err := qnap.New(qnapConfig.URL, s.Logger)
	if err != nil {
		return err
	}

	if err := qnapAPI.Login(qnapConfig.Username, qnapConfig.Password); err != nil {
		return err
	}
	defer qnapAPI.Logout()

	if err := qnapAPI.CreateDir(qnapConfig.DownloadBasePath, dirName); err != nil {
		return err
	}

	return qnapAPI.UploadMany(filepath.Join(qnapConfig.DownloadBasePath, dirName), filePaths, bar)
}
