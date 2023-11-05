package cog

import (
	"context"
	"errors"
	"fmt"

	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/twitter"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

var (
	ErrTwitterLinkNotFound   error = fmt.Errorf("Message does not contain a valid Twitter link!")
	ErrTwitterTweetNotFound  error = fmt.Errorf("Tweet not found!")
	ErrTwitterVideoNotFound  error = fmt.Errorf("Tweet does not contain a video!")
	ErrTwitterPhotosNotFound error = fmt.Errorf("Tweet does not contain photos!")
	ErrTwitterQuotedNotFound error = fmt.Errorf("Tweet does not quote any other tweet!")

	ErrFetchTweet error = fmt.Errorf("Could not fetch this tweet for some reason")
)

type TwitterCog struct {
	GenericCog

	api twitter.API
}

func NewTwitterCog(cfg *config.Config, s *discord.Session) (Cog, error) {
	c := &TwitterCog{}

	cache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		return nil, err
	}

	c.api = twitter.NewCachedAPI(cfg.Twitter, cache, zap.S())

	c.allCommands = map[string]CommandFunc{
		"embed":  c.Embed,
		"photos": c.Photos,
		"video":  c.Video,
		"quoted": c.Quoted,
	}

	return c, nil
}

func (c *TwitterCog) Embed(ctx context.Context, s *discord.MessageSession, args []string) {
	tweet, err := c.getTweetFromArgs(args)
	if err != nil {
		s.SendError(err)
		return
	}

	s.SendEmbeds(tweet.GetEmbeds())
	if tweet.HasVideos() {
		for _, videoURL := range tweet.VideoURLs {
			s.SendVideoURL(videoURL, s.Message.ID)
		}
	}
}

func (c *TwitterCog) Photos(ctx context.Context, s *discord.MessageSession, args []string) {
	tweet, err := c.getTweetFromArgs(args)
	if err != nil {
		s.SendError(err)
		return
	}

	if !tweet.HasPhotos() {
		s.SendError(ErrTwitterPhotosNotFound)
		return
	}

	if len(tweet.PhotoURLs) == 1 {
		return
	}

	s.SendEmbeds(tweet.GetPhotoEmbeds()[1:])
}

func (c *TwitterCog) Video(ctx context.Context, s *discord.MessageSession, args []string) {
	tweet, err := c.getTweetFromArgs(args)
	if err != nil {
		s.SendError(err)
		return
	}

	if !tweet.HasVideos() {
		s.SendError(ErrTwitterVideoNotFound)
		return
	}

	for _, videoURL := range tweet.VideoURLs {
		s.SendVideoURL(videoURL, s.Message.ID)
	}
}

func (c *TwitterCog) Quoted(ctx context.Context, s *discord.MessageSession, args []string) {
	tweet, err := c.getTweetFromArgs(args)
	if err != nil {
		s.SendError(err)
		return
	}

	if !tweet.IsQuoted {
		s.SendError(ErrTwitterQuotedNotFound)
		return
	}

	s.SendEmbeds(tweet.QuotedStatus.GetEmbeds())
	if tweet.QuotedStatus.HasVideos() {
		for _, videoURL := range tweet.QuotedStatus.VideoURLs {
			s.SendVideoURL(videoURL, s.Message.ID)
		}
	}
}

func (c *TwitterCog) getTweetFromArgs(args []string) (*twitter.Tweet, error) {
	if len(args) == 0 {
		return nil, ErrTwitterLinkNotFound
	}

	matches := c.extractTweetIDs(args[0])
	if len(matches) == 0 {
		return nil, ErrTwitterLinkNotFound
	}

	tweet, err := c.api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			return nil, ErrTwitterTweetNotFound
		}
		return nil, ErrFetchTweet
	}

	return tweet, nil
}

func (c *TwitterCog) extractTweetIDs(s string) []string {
	allMatches := twitter.URLRegex.FindAllStringSubmatch(s, -1)

	matches := make([]string, 0, len(allMatches))
	for _, match := range allMatches {
		matches = append(matches, match[1])
	}

	matches = utils.Unique(matches)
	return matches
}
