package cog

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/stalker"
	"go.uber.org/zap"
)

var (
	tweetStalkManager *stalker.TweetStalkManager
)

type TweetStalkCog struct {
	config *config.Config

	DiscordBotCog
}

func (TweetStalkCog) String() string {
	return "tweetstalk"
}

func (c *TweetStalkCog) Setup(ctx context.Context, cfg *config.Config, wg *sync.WaitGroup) error {
	c.DiscordBotCog.Setup(c, cfg, wg)
	c.config = cfg
	c.wg = wg

	return nil
}

func (c *TweetStalkCog) Resume(ctx context.Context, session *discordgo.Session, logger *zap.SugaredLogger) {
	tweetStalkManager = stalker.NewTweetStalkManager(ctx, c.config, session, c.wg, logger)
	tweetStalkManager.Resume(ctx)
}

func (TweetStalkCog) Commands() []Command {
	return []Command{
		&StalkCommand{},
		&UnstalkCommand{},
		&StalksCommand{},
		&ColorCommand{},
	}
}

type StalkCommand struct{}

func (StalkCommand) String() string {
	return "stalk"
}

func (StalkCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	durationStr := args[len(args)-1]

	var screenNames []string
	durationInt, err := strconv.Atoi(durationStr)
	if err != nil {
		screenNames = args
	} else {
		screenNames = args[:len(args)-1]
	}
	duration := time.Duration(durationInt) * time.Minute

	for _, screenName := range screenNames {
		err = tweetStalkManager.Stalk(ctx, channelID, screenName, duration)
		if err != nil {
			_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("Failed to stalk @%s in this channel!", screenName))
			return err
		}

		if durationInt == 0 {
			_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("Stalked @%s in this channel!", screenName))
		} else {
			_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("Stalked @%s in this channel! Will auto-unstalk after %v minutes!", screenName, durationInt))
		}

		if err != nil {
			return err
		}
	}

	return nil
}

type UnstalkCommand struct{}

func (UnstalkCommand) String() string {
	return "unstalk"
}

func (UnstalkCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	for _, screenName := range args {
		err = tweetStalkManager.Unstalk(ctx, channelID, screenName)
		if err != nil {
			_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("Failed to unstalk @%s in this channel!", screenName))
			return err
		}

		_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("Unstalked @%s in this channel!", screenName))
		if err != nil {
			return err
		}
	}

	return nil
}

type StalksCommand struct{}

func (StalksCommand) String() string {
	return "stalks"
}

func (StalksCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	screenNames, err := tweetStalkManager.Stalks(ctx, channelID)
	if err != nil {
		return err
	}

	if len(screenNames) == 0 {
		_, err = session.ChannelMessageSend(channelID, "No users stalked in this channel!")
	} else {
		_, err = session.ChannelMessageSend(channelID, fmt.Sprintf("Users stalked in this channel: %s", strings.Join(screenNames, ", ")))
	}

	return err
}

type ColorCommand struct{}

func (ColorCommand) String() string {
	return "color"
}

func (ColorCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) (err error) {
	if len(args) < 2 {
		session.ChannelMessageSend(channelID, "Insufficient arguments")
		return nil
	}

	screenName, colorStr := args[0], args[1]
	colorInt, err := strconv.ParseInt(colorStr, 16, 0)
	if err != nil {
		session.ChannelMessageSend(channelID, fmt.Sprintf("Invalid color: %s", colorStr))
		return nil
	}

	if err := tweetStalkManager.Color(ctx, channelID, screenName, int(colorInt)); errors.Is(err, stalker.ErrUserNotStalked) {
		session.ChannelMessageSend(channelID, fmt.Sprintf("User @%s not stalked in this channel", screenName))
		return nil
	} else if err != nil {
		session.ChannelMessageSend(channelID, "Failed to set color")
		return err
	}

	_, err = session.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Description: fmt.Sprintf("Set color for @%s", screenName),
		Color:       int(colorInt),
	})
	return err
}
