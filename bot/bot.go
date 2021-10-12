package bot

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/shlex"
	"go.uber.org/zap"

	"github.com/xIceArcher/go-leah/cog"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/handler"
)

type DiscordBot struct {
	Cfg     *config.Config
	OwnerID string
	Prefix  string

	session       *discordgo.Session
	cogMap        map[string]cog.Cog
	handlers      []handler.MessageHandler
	filterRegexes []*regexp.Regexp
}

func New(
	cfg *config.Config,
	cogs []cog.Cog, handlers []handler.MessageHandler,
	intents discordgo.Intent,
) (bot *DiscordBot, err error) {
	filterRegexes := make([]*regexp.Regexp, 0, len(cfg.Discord.FilterRegexes))
	for _, regexStr := range cfg.Discord.FilterRegexes {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("regex %s is invalid", regexStr)
		}

		filterRegexes = append(filterRegexes, regex)
	}

	cogMap := make(map[string]cog.Cog)
	for _, cog := range cogs {
		for _, command := range cog.ActiveCommands() {
			cogMap[command.String()] = cog
		}
	}

	bot = &DiscordBot{
		Cfg:    cfg,
		Prefix: cfg.Discord.Prefix,

		cogMap:        cogMap,
		handlers:      handlers,
		filterRegexes: filterRegexes,
	}

	if bot.session, err = discordgo.New(cfg.Discord.Token); err != nil {
		return nil, err
	}

	bot.session.Identify.Intents = intents
	return bot, nil
}

func (b *DiscordBot) Run(ctx context.Context) error {
	logger := zap.S()
	logger.Info("Starting bot...")

	b.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		logger := logger.With(
			zap.String("guild", m.GuildID),
			zap.String("channel", m.ChannelID),
			zap.String("user", m.Author.Username),
			zap.String("messageID", m.ID),
		)

		// Ignore messages by self
		if m.Author.ID == s.State.User.ID {
			return
		}

		// If message has the prefix, check whether it matches any command
		// Otherwise, run it through all message handlers
		// Essentially, ignore any messages with the prefix that doesn't match any command
		if strings.HasPrefix(m.Content, b.Prefix) {
			b.handleCommand(ctx, s, m, logger)
		} else {
			b.handleMessage(ctx, s, m, logger)
		}
	})

	if err := b.session.Open(); err != nil {
		return err
	}

	logger.Info("Bot started")
	return nil
}

func (b *DiscordBot) Close() {
	logger := zap.S()

	logger.Info("Shutting down bot...")
	b.session.Close()
	logger.Info("Bot shut down")
}

func (b *DiscordBot) Restart(ctx context.Context) error {
	logger := zap.S()
	logger.Info("Restarting bot...")

	intents := b.session.Identify.Intents

	b.Close()
	time.Sleep(5 * time.Second)

	var err error
	if b.session, err = discordgo.New(b.Cfg.Discord.Token); err != nil {
		return err
	}

	b.session.Identify.Intents = intents
	if err := b.Run(ctx); err != nil {
		return err
	}

	logger.Info("Restarted bot")
	return nil
}

func (b *DiscordBot) handleCommand(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, logger *zap.SugaredLogger) {
	msg := m.Content[len(b.Prefix):]
	msgSplit, err := shlex.Split(msg)
	if err != nil {
		logger.With(zap.String("msg", msg)).Error("Failed to parse args")
		return
	}

	msgCommand, msgArgs := msgSplit[0], msgSplit[1:]

	commandLogger := logger.With(
		zap.String("command", msgCommand),
		zap.Strings("args", msgArgs),
	)

	defer func() {
		if r := recover(); r != nil {
			commandLogger.With("panic", r).Error("Command panicked")
		}
	}()

	if msgCommand == (cog.RestartCommand{}).String() {
		if err := b.Restart(ctx); err != nil {
			logger.With(zap.Error(err)).Info("Failed to restart bot")
			panic(err)
		}

		if _, err := s.ChannelMessageSend(m.ChannelID, "Restarted bot!"); err != nil {
			logger.With(zap.Error(err)).Error("Failed to send restart message")
		}

		return
	}

	if c, ok := b.cogMap[msgCommand]; ok {
		if err := c.Handle(ctx, msgCommand, s, m.Message, msgArgs, commandLogger); err != nil {
			switch err {
			case cog.ErrInsufficientPermissions:
				commandLogger.Info("Insufficient permissions")
			default:
				commandLogger.With(zap.Error(err)).Error("Command error")
			}
			return
		}

		commandLogger.Info("Success")
	}
}

func (b *DiscordBot) handleMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, logger *zap.SugaredLogger) {
	for _, h := range b.handlers {
		msg := m.Content
		for _, regex := range b.filterRegexes {
			msg = regex.ReplaceAllLiteralString(msg, "")
		}

		logger := logger.With(
			zap.String("handler", h.String()),
		)

		defer func() {
			if r := recover(); r != nil {
				logger.With("panic", r).Error("Handler panicked")
			}
		}()

		matches, err := h.Handle(s, m.ChannelID, msg, logger)
		logger = logger.With(
			zap.Strings("matches", matches),
		)
		if err != nil {
			logger.With(zap.Error(err)).Error("Handle message")
			continue
		}

		if len(matches) > 0 {
			logger.Info("Success")
		}
	}
}
