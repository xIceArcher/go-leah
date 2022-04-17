package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/shlex"
	"github.com/xIceArcher/go-leah/cog"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type CommandHandler struct {
	GenericHandler

	CommandPrefix string
	AdminID       string

	Cogs []*CogWithConfig

	activeCommands map[string]struct{}
}

type CogWithConfig struct {
	cog.Cog
	*config.DiscordCogConfig
}

func NewCommandHandler(cfg *config.Config, s *discord.Session) (MessageHandler, error) {
	implementedCogs := map[string]cog.Constructor{
		"admin":      cog.NewAdminCog,
		"twitter":    cog.NewTwitterCog,
		"tweetstalk": cog.NewTweetStalkCog,
		"download":   cog.NewDownloadCog,
	}

	activeCommands := make(map[string]struct{})

	cogsWithConfig := make([]*CogWithConfig, 0, len(implementedCogs))
	for cogName, cogCfg := range cfg.Discord.Cogs {
		if len(cogCfg.Commands) == 0 {
			continue
		}

		cogConstructor, ok := implementedCogs[cogName]
		if !ok {
			return nil, fmt.Errorf("cog %s not found", cogName)
		}

		logger := s.Logger.With(zap.String("cog", cogName))

		logger.Info("Initializing cog...")
		c, err := cogConstructor(cfg, s)
		if err != nil {
			return nil, err
		}
		logger.Info("Initialized cog")

		implementedCommands := c.Commands()
		for _, command := range cogCfg.Commands {
			if !slices.Contains(implementedCommands, command) {
				return nil, fmt.Errorf("command %s not found in cog %s", command, cogName)
			}

			if _, ok := activeCommands[command]; ok {
				return nil, fmt.Errorf("duplicate command %s", command)
			}
			activeCommands[command] = struct{}{}
		}

		cogsWithConfig = append(cogsWithConfig, &CogWithConfig{
			Cog:              c,
			DiscordCogConfig: cogCfg,
		})
	}

	return &CommandHandler{
		CommandPrefix: cfg.Discord.Prefix,
		AdminID:       cfg.Discord.AdminID,

		Cogs:           cogsWithConfig,
		activeCommands: activeCommands,
	}, nil
}

func (h *CommandHandler) Handle(ctx context.Context, s *discord.MessageSession) bool {
	if !strings.HasPrefix(s.Content, h.CommandPrefix) {
		return false
	}

	msg := s.Content[len(h.CommandPrefix):]
	if msg == "" {
		return false
	}

	msgSplit, err := shlex.Split(msg)
	if err != nil {
		s.Logger.With(zap.String("msg", msg)).Error("Failed to parse args")
		return false
	}

	msgCommand, msgArgs := msgSplit[0], msgSplit[1:]
	if _, ok := h.activeCommands[msgCommand]; !ok {
		s.Logger.Infof("Command %s not found", msgCommand)
		return true
	}

	s.Logger = s.Logger.With(
		zap.String("command", msgCommand),
		zap.Strings("args", msgArgs),
	)

	for _, cogWithConfig := range h.Cogs {
		if !cogWithConfig.HasCommand(msgCommand) {
			continue
		}

		if cogWithConfig.IsAdminOnly && s.Author.ID != h.AdminID {
			s.Logger.Info("Insufficient permissions")
			return true
		}

		if len(cogWithConfig.ChannelIDs) > 0 && !slices.Contains(cogWithConfig.ChannelIDs, s.ChannelID) {
			s.Logger.Info("Illegal access")
			return true
		}

		cogWithConfig.Handle(ctx, s, msgCommand, msgArgs)
		s.Logger.Info("Success")
		return true
	}

	s.Logger.Infof("Command %s not found", msgCommand)
	return true
}

func (h *CommandHandler) Stop() {
	for _, cog := range h.Cogs {
		cog.Stop()
	}
}
