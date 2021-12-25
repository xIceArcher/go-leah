package twitter

import (
	"context"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type PollStalker struct {
	*twitterStalker

	ctx            context.Context
	api            *API
	userCancelFunc map[string]context.CancelFunc

	PollInterval time.Duration
}

func NewPollStalker(ctx context.Context, cfg *config.TwitterConfig, wg *sync.WaitGroup, logger *zap.SugaredLogger) *PollStalker {
	pollIntervalMins := cfg.PollIntervalMins
	if cfg.PollIntervalMins == 0 {
		pollIntervalMins = 5
	}

	stalker := &PollStalker{
		twitterStalker: newTwitterStalker(logger),

		ctx:            ctx,
		api:            NewAPI(cfg),
		userCancelFunc: make(map[string]context.CancelFunc),

		PollInterval: time.Duration(pollIntervalMins) * time.Minute,
	}

	go stalker.RestartTask(ctx, wg)
	go stalker.CleanupTask(ctx, wg)

	return stalker
}

func (s *PollStalker) AddUsers(userIDs ...string) error {
	for _, userID := range userIDs {
		if s.IsStalkingUser(userID) {
			continue
		}

		sinceID, err := s.api.GetLastTweetID(userID)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(s.ctx)
		s.userCancelFunc[userID] = cancel

		go s.PollTask(ctx, userID, sinceID)
	}

	return s.twitterStalker.AddUsers(userIDs...)
}

func (s *PollStalker) RemoveUsers(userIDs ...string) error {
	for _, userID := range userIDs {
		if !s.IsStalkingUser(userID) {
			continue
		}

		cancelFunc := s.userCancelFunc[userID]
		delete(s.userCancelFunc, userID)

		cancelFunc()
	}

	return s.twitterStalker.RemoveUsers(userIDs...)
}

func (s *PollStalker) RestartTask(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

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

func (s *PollStalker) PollTask(ctx context.Context, userID string, sinceID string) {
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

func (s *PollStalker) CleanupTask(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	<-ctx.Done()
	for _, cancelFunc := range s.userCancelFunc {
		cancelFunc()
	}
}
