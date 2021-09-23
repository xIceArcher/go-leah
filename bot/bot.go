package bot

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/xIceArcher/go-leah/command"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/handler"
)

type DiscordBot struct {
	Cfg     *config.Config
	OwnerID string
	Prefix  string

	session       *discordgo.Session
	commands      []*command.DiscordBotCommand
	handlers      []handler.MessageHandler
	filterRegexes []*regexp.Regexp
}

const (
	CommandRestart = "restart"
)

func New(
	cfg *config.Config,
	commands []*command.DiscordBotCommand, handlers []handler.MessageHandler,
	intents discordgo.Intent,
) (bot *DiscordBot, err error) {
	existingCommands := make(map[string]struct{})
	for _, command := range commands {
		if _, ok := existingCommands[command.Name]; ok {
			return nil, errors.New("duplicate command")
		}

		existingCommands[command.Name] = struct{}{}
	}

	filterRegexes := make([]*regexp.Regexp, 0, len(cfg.Discord.FilterRegexes))
	for _, regexStr := range cfg.Discord.FilterRegexes {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("regex %s is invalid", regexStr)
		}

		filterRegexes = append(filterRegexes, regex)
	}

	bot = &DiscordBot{
		Cfg:    cfg,
		Prefix: cfg.Discord.Prefix,

		commands:      commands,
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
	msgSplit := strings.Split(msg, " ")
	msgCommand, msgArgs := msgSplit[0], msgSplit[1:]

	// Return upon matching any command, should only match one
	for _, command := range b.commands {
		if msgCommand == command.Name {
			commandLogger := logger.With(
				zap.String("command", command.Name),
				zap.Strings("args", msgArgs),
			)

			if command.Configs.IsAdminOnly && m.Author.ID != b.Cfg.Discord.AdminID {
				commandLogger.Info("Invalid user")
				return
			}

			defer func() {
				if r := recover(); r != nil {
					commandLogger.With("panic", r).Error("Command panicked")
				}
			}()

			if msgCommand == CommandRestart {
				if err := b.Restart(ctx); err != nil {
					logger.With(zap.Error(err)).Info("Failed to restart bot")
					panic(err)
				}

				if _, err := s.ChannelMessageSend(m.ChannelID, "Restarted bot!"); err != nil {
					commandLogger.With(zap.Error(err)).Error("Failed to send restart message")
				}

				return
			}

			if err := command.HandlerFunc(ctx, b.Cfg, s, m.Message, msgArgs); err != nil {
				commandLogger.With(zap.Error(err)).Error("Command")
				return
			}

			commandLogger.Info("Success")
			return
		}
	}
}

func (b *DiscordBot) handleMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, logger *zap.SugaredLogger) {
	for _, handler := range b.handlers {
		msg := m.Content
		for _, regex := range b.filterRegexes {
			msg = regex.ReplaceAllLiteralString(msg, "")
		}

		logger := logger.With(
			zap.String("handler", handler.Name()),
		)

		defer func() {
			if r := recover(); r != nil {
				logger.With("panic", r).Error("Handler panicked")
			}
		}()

		matches, err := handler.Handle(s, m.ChannelID, msg, logger)
		if err != nil {
			logger.With(zap.Strings("matches", matches)).With(zap.Error(err)).Error("Handle message")
			continue
		}

		if len(matches) > 0 {
			logger.With(zap.Strings("matches", matches)).Info("Success")
		}
	}
}
