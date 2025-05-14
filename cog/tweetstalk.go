package cog

import (
	"context"

	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/twitter"
	"go.uber.org/zap"
)

type TweetstalkCog struct {
	GenericCog

	cfg          *config.TwitterConfig
	api          twitter.API
	tweetStalker twitter.Stalker

	cancel context.CancelFunc
}

func NewTweetstalkCog(cfg *config.Config, s *discord.Session) (Cog, error) {
	c := &TweetstalkCog{
		cfg: cfg.Twitter,
	}

	cache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		return nil, err
	}

	c.api = twitter.NewCachedAPI(cfg.Twitter, cache, zap.S())

	tweetStalker, err := twitter.NewStalker(cfg.Twitter)
	if err != nil {
		return nil, err
	}

	c.tweetStalker = tweetStalker

	return c, nil
}

func (c *TweetstalkCog) Start(s *discord.Session) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		if err := c.tweetStalker.Stalk(ctx, c.cfg.StalkListID, c.cfg.StalkInterval()); err != nil {
			zap.S().With(zap.Error(err)).Info("Stalker stopped unexpectedly")
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case tweet := <-c.tweetStalker.OutCh():
				if tweet.IsRetweet {
					continue
				}

				if _, err := s.SendEmbeds("1335888187947483147", tweet.GetEmbeds()); err != nil {
					zap.S().With(zap.Error(err)).With(zap.String("id", tweet.ID)).Error("Failed to send embed")
				}
			}
		}
	}()

	return nil
}

func (c *TweetstalkCog) Stop() {
	c.cancel()
}
