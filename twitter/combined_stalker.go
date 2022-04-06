package twitter

import (
	"context"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type CombinedStalker struct {
	stalkers           []Stalker
	sentTweetIDToCount sync.Map
	timeout            time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	restartLock     sync.Mutex
	lastRestartTime time.Time

	outCh  chan *Tweet
	logger *zap.SugaredLogger
}

func NewCombinedStalker(cfg *config.TwitterConfig, stalkers []Stalker, logger *zap.SugaredLogger) *CombinedStalker {
	return &CombinedStalker{
		stalkers: stalkers,
		timeout:  time.Duration(cfg.StalkerTimeoutMins) * time.Minute,

		lastRestartTime: time.Now(),

		outCh:  make(chan *Tweet),
		logger: logger,
	}
}

func (s *CombinedStalker) Start() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	for _, stalker := range s.stalkers {
		if err := stalker.Start(); err != nil {
			return err
		}

		go s.readSingleStalkerChTask(s.ctx, stalker)
	}
	return nil
}

func (s *CombinedStalker) AddUsers(userIDs ...string) (err error) {
	for _, stalker := range s.stalkers {
		if err = stalker.AddUsers(userIDs...); err != nil {
			return err
		}
	}

	return nil
}

func (s *CombinedStalker) RemoveUsers(userIDs ...string) error {
	for _, stalker := range s.stalkers {
		if err := stalker.RemoveUsers(userIDs...); err != nil {
			return err
		}
	}

	return nil
}

func (s *CombinedStalker) Restart() error {
	s.restartLock.Lock()
	defer s.restartLock.Unlock()

	if time.Since(s.lastRestartTime) < 5*time.Minute {
		return nil
	}

	for _, stalker := range s.stalkers {
		if err := stalker.Restart(); err != nil {
			return err
		}
	}

	s.logger.Info("Restarted streams")
	s.lastRestartTime = time.Now()
	return nil
}

func (s *CombinedStalker) IsStalkingUser(userID string) bool {
	return s.stalkers[0].IsStalkingUser(userID)
}

func (s *CombinedStalker) OutCh() <-chan *Tweet {
	return s.outCh
}

func (s *CombinedStalker) readSingleStalkerChTask(ctx context.Context, stalker Stalker) {
	s.wg.Add(1)
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case tweet := <-stalker.OutCh():
			cnt, ok := s.sentTweetIDToCount.Load(tweet.ID)
			if ok {
				s.sentTweetIDToCount.Store(tweet.ID, cnt.(int)+1)
			} else {
				s.sentTweetIDToCount.Store(tweet.ID, 1)
				s.outCh <- tweet

				go s.monitorTweetCountTask(ctx, tweet.ID)
			}
		}
	}
}

func (s *CombinedStalker) monitorTweetCountTask(ctx context.Context, tweetID string) {
	s.wg.Add(1)
	defer s.wg.Done()

	select {
	case <-ctx.Done():
		return
	case <-time.After(s.timeout):
		cnt, ok := s.sentTweetIDToCount.Load(tweetID)
		if ok && cnt.(int) < len(s.stalkers) {
			s.Restart()
		}

		s.sentTweetIDToCount.Delete(tweetID)
	}
}

func (s *CombinedStalker) Stop() {
	s.cancel()

	for _, stalker := range s.stalkers {
		stalker.Stop()
	}

	s.wg.Wait()
}
