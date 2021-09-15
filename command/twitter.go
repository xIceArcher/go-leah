package command

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/twitter"
	"github.com/xIceArcher/go-leah/utils"
)

const (
	twitterLinkNotFound   = "Message does not contain a valid Twitter link!"
	twitterTweetNotFound  = "Tweet not found!"
	twitterVideoNotFound  = "Tweet does not contain a video!"
	twitterPhotosNotFound = "Tweet does not contain photos!"
	twitterQuotedNotFound = "Tweet does not quote any other tweet!"
)

func Embed(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	api := twitter.NewAPI(cfg.Twitter)
	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(msg.ChannelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	_, err = session.ChannelMessageSendEmbeds(msg.ChannelID, tweet.GetEmbeds(cfg.Discord))
	return err
}

func Photos(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	api := twitter.NewAPI(cfg.Twitter)
	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(msg.ChannelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if !tweet.HasPhotos {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterPhotosNotFound)
		return err
	}

	if len(tweet.PhotoURLs) == 1 {
		return nil
	}

	embeds := tweet.GetPhotoEmbeds(cfg.Discord)[1:]
	_, err = session.ChannelMessageSendEmbeds(msg.ChannelID, embeds)
	return err
}

func Video(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	api := twitter.NewAPI(cfg.Twitter)
	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(msg.ChannelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if !tweet.HasVideo {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterVideoNotFound)
		return err
	}

	if _, err = session.ChannelMessageSend(msg.ChannelID, tweet.VideoURL); err != nil {
		return err
	}

	return nil
}

func Quoted(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) (err error) {
	if len(args) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	matches := extractTweetIDs(args[0])
	if len(matches) == 0 {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterLinkNotFound)
		return err
	}

	api := twitter.NewAPI(cfg.Twitter)
	tweet, err := api.GetTweet(matches[0])
	if err != nil {
		if errors.Is(err, twitter.ErrNotFound) {
			_, err = session.ChannelMessageSend(msg.ChannelID, twitterTweetNotFound)
			return err
		}
		return err
	}

	if !tweet.IsQuoted {
		_, err = session.ChannelMessageSend(msg.ChannelID, twitterQuotedNotFound)
		return err
	}

	_, err = session.ChannelMessageSendEmbeds(msg.ChannelID, tweet.QuotedStatus.GetEmbeds(cfg.Discord))
	if err != nil {
		return err
	}

	if !tweet.QuotedStatus.HasVideo {
		return nil
	}

	_, err = session.ChannelMessageSend(msg.ChannelID, tweet.QuotedStatus.VideoURL)
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
