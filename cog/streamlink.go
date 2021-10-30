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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/grafov/m3u8"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jessevdk/go-flags"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/qnap"
	"go.uber.org/zap"
)

var qnapConfig *config.QNAPConfig

type StreamlinkTransport struct {
	client  *retryablehttp.Client
	Headers map[string]string
}

func (t *StreamlinkTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.Headers {
		req.Header.Add(k, v)
	}
	return cleanhttp.DefaultTransport().RoundTrip(req)
}

func NewStreamlinkClient(headers map[string]string) *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.HTTPClient.Timeout = time.Minute
	client.HTTPClient.Transport = &StreamlinkTransport{
		client:  client,
		Headers: headers,
	}
	client.Logger = nil

	return client
}

type DownloadCog struct {
	DiscordBotCog
}

func (DownloadCog) String() string {
	return "download"
}

func (c *DownloadCog) Setup(ctx context.Context, cfg *config.Config) error {
	c.DiscordBotCog.Setup(c, cfg)
	qnapConfig = cfg.QNAP

	return nil
}

func (DownloadCog) Commands() []Command {
	return []Command{
		&StreamlinkCommand{},
	}
}

type StreamlinkCommand struct{}

func (StreamlinkCommand) String() string {
	return "streamlink"
}

type StreamlinkCommandArgs struct {
	HTTPHeader  string `short:"h" long:"header"`
	HTTPCookies string `short:"c" long:"cookie"`

	Args struct {
		M3U8URLStr string `required:"yes"`
		FileName   string `required:"yes"`
	} `positional-args:"yes"`
}

func (c *StreamlinkCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	commandArgs := &StreamlinkCommandArgs{}
	_, err = flags.NewParser(commandArgs, flags.IgnoreUnknown).ParseArgs(args)
	if err != nil {
		return err
	}

	client := NewStreamlinkClient(parseHeaders(commandArgs))

	m3u8Url, err := url.Parse(commandArgs.Args.M3U8URLStr)
	if err != nil {
		return err
	}

	var listType m3u8.ListType
	defer func() {
		if err != nil {
			session.ChannelMessageSend(channelID, fmt.Sprintf("Failed to complete streamlink %s, error: %s", commandArgs.Args.FileName, err.Error()))
			return
		}

		if listType == m3u8.MEDIA {
			session.ChannelMessageSend(channelID, fmt.Sprintf("Successfully completed streamlink %s", commandArgs.Args.FileName))
		}
	}()

	resp, err := client.Get(commandArgs.Args.M3U8URLStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request to %s returned status %v", resp.Request.URL.String(), resp.StatusCode)
	}

	playlist, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return err
	}

	var downloadedFiles []string
	switch listType {
	case m3u8.MEDIA:
		dir := fmt.Sprintf("%s-%s", time.Now().Format(consts.TimeFormatYYMMDDHHMMSS), channelID)
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			return err
		}

		mediaList := playlist.(*m3u8.MediaPlaylist)

		session.ChannelMessageSend(channelID, fmt.Sprintf("Starting to download %s", commandArgs.Args.FileName))

		downloadedFiles, err = handleMediaPlaylist(ctx, client, m3u8Url, mediaList.Key, dir, logger)
		if err != nil {
			return err
		}
	case m3u8.MASTER:
		var bestVariant *m3u8.Variant
		for _, variant := range playlist.(*m3u8.MasterPlaylist).Variants {
			if bestVariant == nil || variant.Bandwidth > bestVariant.Bandwidth {
				bestVariant = variant
			}
		}

		mediaUrl, err := m3u8Url.Parse(bestVariant.URI)
		if err != nil {
			return err
		}

		args[len(args)-2] = mediaUrl.String()
		return c.Handle(ctx, session, channelID, args, logger)
	default:
		return fmt.Errorf("unknown M3U8 type %v", listType)
	}

	if qnapConfig.IsEnabled {
		session.ChannelMessageSend(channelID, fmt.Sprintf("Stream closed, starting to upload %s", commandArgs.Args.FileName))
		return uploadFile(commandArgs.Args.FileName, downloadedFiles, logger)
	}

	return nil
}

type DownloadedFile struct {
	Name  string
	SeqNo int
}

func handleMediaPlaylist(ctx context.Context, client *retryablehttp.Client, m3u8Url *url.URL, key *m3u8.Key, dir string, logger *zap.SugaredLogger) (downloadFiles []string, err error) {
	isEncrypted := key != nil

	var block cipher.Block
	if isEncrypted {
		keyBytes, err := downloadKey(ctx, client, m3u8Url, key)
		if err != nil {
			return nil, err
		}

		block, err = aes.NewCipher(keyBytes)
		if err != nil {
			logger.Error(err)
			return nil, err
		}
	}

	var wg sync.WaitGroup
	segmentUrlChan := make(chan *Segment)
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

	downloadedSegments := make(map[int]string)
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
			files := make([]*DownloadedFile, 0, len(downloadedSegments))
			for seqNo, filePath := range downloadedSegments {
				files = append(files, &DownloadedFile{
					Name:  filePath,
					SeqNo: seqNo,
				})
			}
			sort.Slice(files, func(i, j int) bool {
				return files[i].SeqNo < files[j].SeqNo
			})

			downloadFiles = make([]string, 0, len(files))
			for _, file := range files {
				downloadFiles = append(downloadFiles, file.Name)
			}
			return downloadFiles, nil
		case <-time.After(sleepTime):
			if func() error {
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

					seqNo := i + int(mediaList.SeqNo)
					if _, ok := downloadedSegments[seqNo]; ok {
						continue
					}

					segmentUrl, err := m3u8Url.Parse(segment.URI)
					if err != nil {
						return err
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

					fileName := filepath.Join(dir, path.Base(segmentUrl.Path))
					segmentUrlChan <- &Segment{
						FileName: fileName,
						URL:      segmentUrl,
						IV:       iv,
					}

					downloadedSegments[seqNo] = fileName
				}

				if mediaList.Closed {
					doneCh <- 1
				}

				return nil
			}(); err != nil {
				logger.Error(err)
				errCount++

				if errCount > 10 {
					logger.Error("Too many errors when getting M3U8, aborting...")
					doneCh <- 1
				}
			}

			errCount = 0
		}
	}
}

func parseHeaders(args *StreamlinkCommandArgs) map[string]string {
	headers := make(map[string]string)
	if args.HTTPHeader != "" {
		headerList := strings.Split(args.HTTPHeader, ";")
		for _, header := range headerList {
			headerKeyVal := strings.Split(header, "=")
			if len(headerKeyVal) != 2 {
				continue
			}

			key, val := headerKeyVal[0], headerKeyVal[1]
			headers[key] = val
		}
	}

	if args.HTTPCookies != "" {
		headers["Cookie"] = args.HTTPCookies
	}

	return headers
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

type PlaylistConfig struct {
	Directory   string
	IsEncrypted bool
	Block       cipher.Block
}

type Segment struct {
	FileName   string
	URL        *url.URL
	IV         []byte
	NumRetries int
}

func downloadSegment(ctx context.Context, cfg *PlaylistConfig, client *retryablehttp.Client, segmentChan chan *Segment, logger *zap.SugaredLogger) {
	for {
		select {
		case <-ctx.Done():
			return
		case segment, ok := <-segmentChan:
			if !ok {
				return
			}

			segmentLogger := logger.With(zap.String("url", segment.URL.String()))
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
				segmentLogger.Error(err)

				if segment.NumRetries > 5 {
					segmentLogger.Error("Exceeded number of retries")
					continue
				}

				segment.NumRetries++
				segmentChan <- segment
			}
		}
	}
}

func uploadFile(fileName string, filePaths []string, logger *zap.SugaredLogger) error {
	if !qnapConfig.IsEnabled {
		return nil
	}

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
