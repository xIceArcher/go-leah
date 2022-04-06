package bot

import (
	"context"
	"fmt"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/handler"
	"go.uber.org/zap"
)

type Bot struct {
	Session *discord.Session

	ctx    context.Context
	cancel context.CancelFunc

	messageHandlers          handler.MessageHandlers
	messageHandlerCancelFunc func()

	// Global settings
	filterRegexes []*regexp.Regexp
}

func New(cfg *config.Config, intents discordgo.Intent, logger *zap.SugaredLogger) (*Bot, error) {
	filterRegexes := make([]*regexp.Regexp, 0, len(cfg.Discord.FilterRegexes))
	for _, regexStr := range cfg.Discord.FilterRegexes {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("regex %s is invalid", regexStr)
		}

		filterRegexes = append(filterRegexes, regex)
	}

	session, err := discordgo.New(cfg.Discord.Token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = intents

	return &Bot{
		Session: discord.NewSession(session, logger),

		filterRegexes: filterRegexes,
	}, nil
}

func (b *Bot) Start() error {
	b.ctx, b.cancel = context.WithCancel(context.Background())

	return b.Session.Open()
}

func (b *Bot) AddHandler(h handler.MessageHandler) {
	b.messageHandlers = append(b.messageHandlers, h)

	if b.messageHandlerCancelFunc != nil {
		b.messageHandlerCancelFunc()
	}

	b.messageHandlerCancelFunc = b.Session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		logger := zap.S().With(
			zap.String("guild", m.GuildID),
			zap.String("channel", m.ChannelID),
			zap.String("user", m.Author.Username),
			zap.String("messageID", m.ID),
		)

		// Ignore messages by self
		if m.Author.ID == s.State.User.ID {
			return
		}

		b.messageHandlers.HandleOne(b.ctx, discord.NewMessageSession(s, m.Message, logger))
	})
}

func (b *Bot) Stop() {
	b.cancel()
}
