package cog

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
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

		s.SendMessage("Starting to download %s", commandArgs.Args.FileName)

		downloadedRuns, err := handleMediaPlaylist(ctx, client, commandArgs.Args.M3U8URLStr, playlist.Key, dir, s.Logger)
		if err != nil {
			s.SendError(err)
			return
		}

		if c.qnapConfig.IsEnabled {
			s.SendMessage("Stream closed, starting to upload %s", commandArgs.Args.FileName)

			if err := handleUpload(c.qnapConfig, commandArgs.Args.FileName, downloadedRuns, s.Logger); err != nil {
				s.SendError(err)
				return
			}
		}
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

func handleMediaPlaylist(ctx context.Context, client *retryablehttp.Client, m3u8UrlStr string, key *m3u8.Key, dir string, logger *zap.SugaredLogger) (downloadRuns [][]string, err error) {
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

	var wg sync.WaitGroup
	segmentUrlChan := make(chan *Segment, 10000)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadSegment(ctx, &PlaylistConfig{
				Directory:   dir,
				IsEncrypted: isEncrypted,
				Block:       block,
			}, client, segmentUrlChan, logger)
		}()
	}

	defer func() {
		close(segmentUrlChan)
		wg.Wait()
	}()

	downloadedRuns := make([]map[int]string, 0)
	downloadedRuns = append(downloadedRuns, make(map[int]string))

	doneCh := make(chan int, 1)

	var sleepTime time.Duration
	errCount := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, download aborted")
			return nil, nil
		case <-doneCh:
			logger.Info("Stream closed")
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
						if mediaList.Key.IV == "" {
							iv = make([]byte, 16)
							binary.BigEndian.PutUint16(iv, uint16(i+int(mediaList.SeqNo)))
						} else {
							iv = []byte(mediaList.Key.IV)
						}
					}

					segmentUrlChan <- &Segment{
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
				logger.With(zap.Int("errCount", errCount)).Error(err)
				errCount++

				if errCount >= 10 {
					logger.Error("Too many errors when getting M3U8, aborting...")
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

func downloadSegment(ctx context.Context, cfg *PlaylistConfig, client *retryablehttp.Client, segmentChan <-chan *Segment, logger *zap.SugaredLogger) {
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

func handleUpload(qnapConfig *config.QNAPConfig, fileNameStr string, downloadedRuns [][]string, logger *zap.SugaredLogger) error {
	extension := path.Ext(fileNameStr)
	fileName := strings.TrimSuffix(fileNameStr, extension)

	if extension == "" {
		extension = ".ts"
	}

	if len(downloadedRuns) == 1 {
		return uploadFile(qnapConfig, fmt.Sprintf("%s%s", fileName, extension), downloadedRuns[0], logger)
	} else {
		for runNo, run := range downloadedRuns {
			runFileName := fmt.Sprintf("%s_%v%s", fileName, runNo+1, extension)
			if err := uploadFile(qnapConfig, runFileName, run, logger); err != nil {
				return err
			}
		}
		return nil
	}
}

func uploadFile(qnapConfig *config.QNAPConfig, fileName string, filePaths []string, logger *zap.SugaredLogger) error {
	qnapAPI, err := qnap.New(qnapConfig.URL, logger)
	if err != nil {
		return err
	}

	if err := qnapAPI.Login(qnapConfig.Username, qnapConfig.Password); err != nil {
		return err
	}
	defer qnapAPI.Logout()

	return qnapAPI.UploadMany(qnapConfig.DownloadBasePath, fileName, filePaths)
}
