package twitter

import (
	"context"
	"sync"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type StreamStalker struct {
	*twitterStalker

	MaxRestartRetries int

	stream *twitter.Stream
	demux  twitter.SwitchDemux
}

var (
	streamStalker          *StreamStalker
	streamStalkerSetupOnce sync.Once
)

func NewStreamStalker(ctx context.Context, cfg *config.TwitterConfig, wg *sync.WaitGroup, logger *zap.SugaredLogger) *StreamStalker {
	api := NewAPI(cfg)

	streamStalkerSetupOnce.Do(func() {
		streamStalker = &StreamStalker{
			twitterStalker: newTwitterStalker(logger),

			MaxRestartRetries: cfg.MaxStreamRestartRetries,
			demux:             twitter.NewSwitchDemux(),
		}

		streamStalker.demux.Tweet = func(t *twitter.Tweet) {
			if _, ok := streamStalker.userIDs[t.User.IDStr]; !ok {
				return
			}

			tweet, err := api.GetTweet(t.IDStr)
			if err != nil {
				logger.With(zap.Error(err), zap.String("tweetID", t.IDStr)).Error("Failed to get tweet")
				return
			}

			streamStalker.outCh <- tweet
		}

		streamStalker.demux.StreamDisconnect = func(disconnect *twitter.StreamDisconnect) {
			logger.With(zap.String("reason", disconnect.Reason)).Warn("Stream crashed")

			streamStalker.restartCh <- 1
			if err := <-streamStalker.restartErrCh; err != nil {
				streamStalker.logger.With(zap.Error(err)).Error("Stream crashed and cannot be recovered")
			}
		}

		go streamStalker.RestartTask(ctx, wg)
		go streamStalker.CleanupTask(ctx, wg)
	})

	return streamStalker
}

func (s *StreamStalker) Start() (err error) {
	if len(s.userIDs) == 0 {
		return nil
	}

	userIDs := make([]string, 0, len(s.userIDs))
	for userID := range s.userIDs {
		userIDs = append(userIDs, userID)
	}

	params := &twitter.StreamFilterParams{
		Follow:        userIDs,
		StallWarnings: twitter.Bool(true),
	}

	s.stream, err = api.Streams.Filter(params)
	if err != nil {
		return err
	}

	go s.demux.HandleChan(s.stream.Messages)
	return nil
}

func (s *StreamStalker) Stop() {
	if s.stream != nil {
		s.stream.Stop()
	}
	s.stream = nil
}

func (s *StreamStalker) RestartTask(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.restartCh:
			currRestartRetries := 0
			var err error

			s.Stop()

			for currRestartRetries < s.MaxRestartRetries {
				if err = s.Start(); err != nil {
					s.logger.With(zap.Error(err)).Error("Failed to restart stream")

					time.Sleep((1 << currRestartRetries) * time.Second)
					currRestartRetries += 1
					continue
				}

				s.logger.Info("Restarted stream")
				break
			}

			s.restartErrCh <- err
		}
	}
}

func (s *StreamStalker) CleanupTask(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	<-ctx.Done()
	s.Stop()
}
