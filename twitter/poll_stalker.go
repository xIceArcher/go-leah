package twitter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type PollStalker struct {
	*twitterStalker

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	isStarted bool

	api            API
	userCancelFunc map[string]context.CancelFunc

	PollInterval time.Duration
}

func NewPollStalker(cfg *config.TwitterConfig, api API, logger *zap.SugaredLogger) *PollStalker {
	pollIntervalMins := cfg.PollIntervalMins
	if cfg.PollIntervalMins == 0 {
		pollIntervalMins = 5
	}

	stalker := &PollStalker{
		twitterStalker: newTwitterStalker(logger),

		api:            api,
		userCancelFunc: make(map[string]context.CancelFunc),

		PollInterval: time.Duration(pollIntervalMins) * time.Minute,
	}

	return stalker
}

func (s *PollStalker) Start() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	for userID := range s.twitterStalker.userIDs {
		if err := s.startStalk(userID); err != nil {
			return err
		}
	}

	go s.restartTask(s.ctx)

	s.isStarted = true
	return nil
}

func (s *PollStalker) AddUsers(userIDs ...string) error {
	for _, userID := range userIDs {
		if s.IsStalkingUser(userID) {
			continue
		}

		if s.isStarted {
			if err := s.startStalk(userID); err != nil {
				return err
			}
		}
	}

	return s.twitterStalker.AddUsers(userIDs...)
}

func (s *PollStalker) RemoveUsers(userIDs ...string) error {
	for _, userID := range userIDs {
		if !s.IsStalkingUser(userID) {
			continue
		}

		if s.isStarted {
			if err := s.stopStalk(userID); err != nil {
				return err
			}
		}
	}

	return s.twitterStalker.RemoveUsers(userIDs...)
}

func (s *PollStalker) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *PollStalker) startStalk(userID string) error {
	sinceID, err := s.api.GetLastTweetID(userID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.userCancelFunc[userID] = cancel

	go s.pollTask(ctx, userID, sinceID)
	return nil
}

func (s *PollStalker) stopStalk(userID string) error {
	cancelFunc, ok := s.userCancelFunc[userID]
	if !ok {
		return fmt.Errorf("stalk cancel func not found")
	}

	delete(s.userCancelFunc, userID)
	cancelFunc()
	return nil
}

func (s *PollStalker) restartTask(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.restartCh:
			// No-op since there's nothing to restart
			s.restartErrCh <- nil
		}
	}
}

func (s *PollStalker) pollTask(ctx context.Context, userID string, sinceID string) {
	s.wg.Add(1)
	defer s.wg.Done()

	logger := s.logger.With(zap.String("userID", userID))

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.PollInterval):
			tweets, err := s.api.GetUserTimeline(userID, sinceID)
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to poll tweets")
				continue
			}

			if len(tweets) == 0 {
				continue
			}

			for i := len(tweets) - 1; i >= 0; i-- {
				s.outCh <- tweets[i]
			}

			sinceID = tweets[0].ID
		}
	}
}
