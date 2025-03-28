package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"time"

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
	session.Client.Timeout = time.Minute

	if cfg.Discord.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.Discord.ProxyURL)
		if err != nil {
			return nil, err
		}

		session.Client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

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
		// Ignore messages by self
		if m.Author.ID == s.State.User.ID {
			return
		}

		logger := zap.S()

		guild, err := s.State.Guild(m.GuildID)
		if err != nil {
			logger = logger.With("guildID", m.GuildID)
		} else {
			logger = logger.With("guild", guild.Name)
		}

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			logger = logger.With("channelID", m.ChannelID)
		} else {
			logger = logger.With("channel", channel.Name)
		}

		logger = logger.With(
			zap.String("user", m.Author.Username),
			zap.String("messageID", m.ID),
		)

		defer func() {
			if r := recover(); r != nil {
				logger.With("reason", r).With("stackTrace", string(debug.Stack())).Error("Command panicked")
			}
		}()

		b.messageHandlers.HandleOne(b.ctx, discord.NewMessageSession(s, m.Message, logger))
	})
}

func (b *Bot) Stop() {
	b.cancel()
}
