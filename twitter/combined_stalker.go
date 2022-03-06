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

	restartLock     sync.Mutex
	lastRestartTime time.Time

	outCh  chan *Tweet
	logger *zap.SugaredLogger
}

func NewCombinedStalker(ctx context.Context, cfg *config.TwitterConfig, stalkers []Stalker, logger *zap.SugaredLogger) *CombinedStalker {
	s := &CombinedStalker{
		stalkers: stalkers,
		timeout:  time.Duration(cfg.StalkerTimeoutMins) * time.Minute,

		lastRestartTime: time.Now(),

		outCh:  make(chan *Tweet),
		logger: logger,
	}

	s.StartReadChTask()
	return s
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

func (s *CombinedStalker) StartReadChTask() {
	for i := range s.stalkers {
		go s.ReadSingleStalkerChTask(i)
	}
}

func (s *CombinedStalker) ReadSingleStalkerChTask(i int) {
	stalker := s.stalkers[i]

	for tweet := range stalker.OutCh() {
		cnt, ok := s.sentTweetIDToCount.Load(tweet.ID)
		if ok {
			s.sentTweetIDToCount.Store(tweet.ID, cnt.(int)+1)
		} else {
			s.sentTweetIDToCount.Store(tweet.ID, 1)
			s.outCh <- tweet

			go s.MonitorTweetCountTask(tweet.ID)
		}
	}
}

func (s *CombinedStalker) MonitorTweetCountTask(tweetID string) {
	time.Sleep(s.timeout)

	cnt, ok := s.sentTweetIDToCount.Load(tweetID)
	if ok && cnt.(int) < len(s.stalkers) {
		s.Restart()
	}

	s.sentTweetIDToCount.Delete(tweetID)
}
