package cog

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/stalker"
	"go.uber.org/zap"
)

type TweetStalkCog struct {
	GenericCog

	tweetStalkManager *stalker.TweetStalkManager
}

func NewTweetStalkCog(cfg *config.Config, s *discord.Session) (Cog, error) {
	c := &TweetStalkCog{}

	c.allCommands = map[string]CommandFunc{
		"stalk":   c.Stalk,
		"unstalk": c.Unstalk,
		"stalks":  c.Stalks,
		"color":   c.Color,
	}

	c.tweetStalkManager = stalker.NewTweetStalkManager(cfg, s, zap.S())
	if err := c.tweetStalkManager.Start(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *TweetStalkCog) Stalk(ctx context.Context, s *discord.MessageSession, args []string) {
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
		err = c.tweetStalkManager.Stalk(s.ChannelID, screenName, duration)
		if err != nil {
			s.SendInternalErrorWithMessage(err, "Failed to stalk @%s in this channel!", screenName)
		}

		if durationInt == 0 {
			s.SendMessage("Stalked @%s in this channel!", screenName)
		} else {
			s.SendMessage("Stalked @%s in this channel! Will auto-unstalk after %v minutes!", screenName, durationInt)
		}
	}
}

func (c *TweetStalkCog) Unstalk(ctx context.Context, s *discord.MessageSession, args []string) {
	for _, screenName := range args {
		if err := c.tweetStalkManager.Unstalk(s.ChannelID, screenName); err != nil {
			s.SendInternalErrorWithMessage(err, "Failed to unstalk @%s in this channel!", screenName)
			return
		}

		s.SendMessage("Unstalked @%s in this channel!", screenName)
	}

}

func (c *TweetStalkCog) Stalks(ctx context.Context, s *discord.MessageSession, args []string) {
	screenNames, err := c.tweetStalkManager.Stalks(s.ChannelID)
	if err != nil {
		s.SendInternalError(err)
		return
	}

	if len(screenNames) == 0 {
		s.SendMessage("No users stalked in this channel!")
	} else {
		s.SendMessage("Users stalked in this channel: %s", strings.Join(screenNames, ", "))
	}
}

func (c *TweetStalkCog) Color(ctx context.Context, s *discord.MessageSession, args []string) {
	if len(args) < 2 {
		s.SendErrorf("Insufficient arguments")
		return
	}

	screenName, colorStr := args[0], args[1]
	colorInt, err := strconv.ParseInt(colorStr, 16, 0)
	if err != nil {
		s.SendErrorf("Invalid color: %s", colorStr)
		return
	}

	if err := c.tweetStalkManager.Color(s.ChannelID, screenName, int(colorInt)); errors.Is(err, stalker.ErrUserNotStalked) {
		s.SendErrorf("User @%s not stalked in this channel", screenName)
		return
	} else if err != nil {
		s.SendInternalError(err)
		return
	}

	s.SendEmbed(&discordgo.MessageEmbed{
		Description: fmt.Sprintf("Set color for @%s", screenName),
		Color:       int(colorInt),
	})
}

func (c *TweetStalkCog) Stop() {
	c.tweetStalkManager.Stop()
}
