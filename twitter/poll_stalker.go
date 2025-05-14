package twitter

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/xIceArcher/go-leah/config"
)

type Stalker interface {
	Stalk(ctx context.Context, listID string, interval time.Duration) error
	OutCh() <-chan *Tweet
}

type StalkerImpl struct {
	api API
	ch  chan *Tweet
}

func NewStalker(c *config.TwitterConfig) (*StalkerImpl, error) {
	return &StalkerImpl{
		api: NewBaseAPI(c),
		ch:  make(chan *Tweet),
	}, nil
}

func (s *StalkerImpl) Stalk(ctx context.Context, listID string, interval time.Duration) error {
	since := time.Now()

	for c := time.Tick(interval); ; {
		nextSince, err := s.RefreshList(ctx, listID, since)
		if err != nil {
			zap.S().With(zap.Error(err)).Error("Failed to refresh list")
			continue
		}

		since = nextSince

		select {
		case <-ctx.Done():
			close(s.ch)
			return nil
		case <-c:
			continue
		}
	}
}

func (s *StalkerImpl) RefreshList(ctx context.Context, listID string, since time.Time) (nextSince time.Time, err error) {
	logger := zap.S()

	tweets, err := s.api.ListTweetsFromList(listID, since)
	if err != nil {
		return since, err
	}

	for _, tweet := range tweets {
		s.ch <- tweet
		logger.With(zap.String("tweet", tweet.URL())).Info("Sent tweet")
	}

	if len(tweets) > 0 {
		nextSince = tweets[len(tweets)-1].Timestamp.Add(time.Second)
	} else {
		nextSince = since
	}

	logger.With(zap.Time("nextSince", nextSince)).Infof("Sent %v tweets", len(tweets))
	return nextSince, nil
}

func (s *StalkerImpl) OutCh() <-chan *Tweet {
	return s.ch
}
