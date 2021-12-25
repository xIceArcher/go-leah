package cog

import (
	"context"
	"errors"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/twitter"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

const (
	twitterLinkNotFound   = "Message does not contain a valid Twitter link!"
	twitterTweetNotFound  = "Tweet not found!"
	twitterVideoNotFound  = "Tweet does not contain a video!"
	twitterPhotosNotFound = "Tweet does not contain photos!"
	twitterQuotedNotFound = "Tweet does not quote any other tweet!"
)

var api twitter.API

type TwitterCog struct {
	DiscordBotCog
}

func (TwitterCog) String() string {
	return "twitter"
}

func (c *TwitterCog) Setup(ctx context.Context, cfg *config.Config, wg *sync.WaitGroup) error {
	c.DiscordBotCog.Setup(c, cfg, wg)

	cache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		return err
	}

	api = twitter.NewCachedAPI(cfg.Twitter, cache, zap.S())
	return nil
}

func (c *TwitterCog) Resume(ctx context.Context, session *discordgo.Session, logger *zap.SugaredLogger) {
}

func (TwitterCog) Commands() []Command {
	return []Command{
		&EmbedCommand{},
		&PhotosCommand{},
		&VideoCommand{},
		&QuotedCommand{},
	}
}

type EmbedCommand struct{}

func (EmbedCommand) String() string {
	return "embed"
}

func (EmbedCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(channelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if _, err = session.ChannelMessageSendEmbeds(channelID, tweet.GetEmbeds()); err != nil {
		return err
	}

	if tweet.HasVideo {
		if _, err = session.ChannelMessageSend(channelID, tweet.VideoURL); err != nil {
			return err
		}
	}

	return nil
}

type PhotosCommand struct{}

func (PhotosCommand) String() string {
	return "photos"
}

func (PhotosCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(channelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if !tweet.HasPhotos {
		_, err = session.ChannelMessageSend(channelID, twitterPhotosNotFound)
		return err
	}

	if len(tweet.PhotoURLs) == 1 {
		return nil
	}

	embeds := tweet.GetPhotoEmbeds()[1:]
	_, err = session.ChannelMessageSendEmbeds(channelID, embeds)
	return err
}

type VideoCommand struct{}

func (VideoCommand) String() string {
	return "video"
}

func (VideoCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(channelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if !tweet.HasVideo {
		_, err = session.ChannelMessageSend(channelID, twitterVideoNotFound)
		return err
	}

	if _, err = session.ChannelMessageSend(channelID, tweet.VideoURL); err != nil {
		return err
	}

	return nil
}

type QuotedCommand struct{}

func (QuotedCommand) String() string {
	return "quoted"
}

func (QuotedCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(channelID, twitterLinkNotFound)
		return err
	}

	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(channelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if !tweet.IsQuoted {
		_, err = session.ChannelMessageSend(channelID, twitterQuotedNotFound)
		return err
	}

	_, err = session.ChannelMessageSendEmbeds(channelID, tweet.QuotedStatus.GetEmbeds())
	if err != nil {
		return err
	}

	if !tweet.QuotedStatus.HasVideo {
		return nil
	}

	_, err = session.ChannelMessageSend(channelID, tweet.QuotedStatus.VideoURL)
	return err
}

func extractTweetIDs(s string) []string {
	allMatches := twitter.URLRegex.FindAllStringSubmatch(s, -1)

	matches := make([]string, 0, len(allMatches))
	for _, match := range allMatches {
		matches = append(matches, match[1])
	}

	matches = utils.Unique(matches)
	return matches
}
