package twitter

import (
	"context"
	"sync"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type UserStalker struct {
	Cfg               *config.TwitterConfig
	UserIDs           map[string]struct{}
	OutCh             chan *Tweet
	MaxRestartRetries int

	stream *twitter.Stream
	demux  twitter.SwitchDemux

	logger *zap.SugaredLogger
}

var (
	userStalker          *UserStalker
	userStalkerSetupOnce sync.Once
)

func NewUserStalker(ctx context.Context, cfg *config.TwitterConfig, wg *sync.WaitGroup, logger *zap.SugaredLogger) *UserStalker {
	api := NewAPI(cfg)

	userStalkerSetupOnce.Do(func() {
		userStalker = &UserStalker{
			Cfg:               cfg,
			UserIDs:           make(map[string]struct{}),
			OutCh:             make(chan *Tweet),
			MaxRestartRetries: cfg.MaxStreamRestartRetries,

			demux:  twitter.NewSwitchDemux(),
			logger: logger,
		}

		userStalker.demux.Tweet = func(t *twitter.Tweet) {
			if _, ok := userStalker.UserIDs[t.User.IDStr]; !ok {
				return
			}

			tweet, err := api.GetTweet(t.IDStr)
			if err != nil {
				logger.With(zap.Error(err), zap.String("tweetID", t.IDStr)).Error("Failed to get tweet")
				return
			}

			userStalker.OutCh <- tweet
		}

		userStalker.demux.StreamDisconnect = func(disconnect *twitter.StreamDisconnect) {
			logger.Warn("Stream crashed")
			userStalker.Restart()
		}

		go userStalker.Cleanup(ctx, wg)
	})

	return userStalker
}

func (s *UserStalker) AddUsers(userIDs ...string) error {
	shouldRestart := false
	for _, userID := range userIDs {
		if s.IsStalkingUser(userID) {
			continue
		}

		shouldRestart = true
		s.UserIDs[userID] = struct{}{}
	}

	if shouldRestart {
		return s.Restart()
	}

	return nil
}

func (s *UserStalker) RemoveUsers(userIDs ...string) error {
	shouldRestart := false
	for _, userID := range userIDs {
		if !s.IsStalkingUser(userID) {
			continue
		}

		shouldRestart = true
		delete(s.UserIDs, userID)
	}

	if shouldRestart {
		return s.Restart()
	}

	return nil
}

func (s *UserStalker) IsStalkingUser(userID string) bool {
	_, ok := s.UserIDs[userID]
	return ok
}

func (s *UserStalker) Restart() (err error) {
	s.Stop()

	currRestartRetries := 0
	for currRestartRetries < s.MaxRestartRetries {
		if err = s.Start(); err != nil {
			s.logger.With(zap.Error(err)).Error("Failed to restart stream")

			time.Sleep((1 << currRestartRetries) * time.Second)
			currRestartRetries += 1
			continue
		}

		break
	}

	s.logger.Info("Restarted stream")
	return
}

func (s *UserStalker) Start() (err error) {
	if len(s.UserIDs) == 0 {
		return nil
	}

	userIDs := make([]string, 0, len(s.UserIDs))
	for userID := range s.UserIDs {
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

func (s *UserStalker) Stop() {
	if s.stream != nil {
		s.stream.Stop()
	}
	s.stream = nil
}

func (s *UserStalker) Cleanup(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	<-ctx.Done()
	s.Stop()
}
