package cog

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

var implementedCogs []Cog = []Cog{
	&AdminCog{},
	&TwitterCog{},
	&DownloadCog{},
}

var (
	ErrNotImplemented          error = fmt.Errorf("not implemented")
	ErrInsufficientPermissions error = fmt.Errorf("insufficient permissions")
	ErrIllegalAccess           error = fmt.Errorf("illegal access")
)

type Cog interface {
	Setup(ctx context.Context, cfg *config.Config) error
	Handle(ctx context.Context, command string, session *discordgo.Session, msg *discordgo.Message, args []string, logger *zap.SugaredLogger) error

	Commands() []Command

	IsCommandActive(command string) bool
	ActiveCommands() []Command

	ActivateCommand(command string) error

	String() string
}

type DiscordBotCog struct {
	cog     Cog
	cogCfg  *config.DiscordCogConfig
	adminID string

	activeCommandMap map[string]Command
}

func (c *DiscordBotCog) Setup(cog Cog, cfg *config.Config) {
	c.cog = cog
	c.activeCommandMap = make(map[string]Command)

	c.cogCfg = cfg.Discord.Cogs[cog.String()]
	c.adminID = cfg.Discord.AdminID
}

func (c *DiscordBotCog) Handle(ctx context.Context, command string, session *discordgo.Session, msg *discordgo.Message, args []string, logger *zap.SugaredLogger) error {
	if c.IsCommandActive(command) {
		if c.cogCfg.IsAdminOnly && msg.Author.ID != c.adminID {
			return ErrInsufficientPermissions
		}

		if len(c.cogCfg.ChannelIDs) > 0 && !utils.Contains(c.cogCfg.ChannelIDs, msg.ChannelID) {
			return ErrIllegalAccess
		}

		return c.activeCommandMap[command].Handle(ctx, session, msg.ChannelID, args, logger)
	}

	return nil
}

func (c *DiscordBotCog) ActiveCommands() []Command {
	activeCommands := make([]Command, 0, len(c.activeCommandMap))
	for _, command := range c.activeCommandMap {
		activeCommands = append(activeCommands, command)
	}
	return activeCommands
}

func (c *DiscordBotCog) IsCommandActive(command string) bool {
	_, ok := c.activeCommandMap[command]
	return ok
}

func (c *DiscordBotCog) ActivateCommand(commandName string) error {
	for _, command := range c.cog.Commands() {
		if command.String() == commandName {
			c.activeCommandMap[commandName] = command
			return nil
		}
	}

	return ErrNotImplemented
}

type Command interface {
	Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) error
	String() string
}

func SetupCogs(ctx context.Context, cfg *config.Config, logger *zap.SugaredLogger) (cogs []Cog, err error) {
	availableCogs := make(map[string]Cog)
	for _, cog := range implementedCogs {
		availableCogs[cog.String()] = cog
	}

	existingCommands := make(map[string]Cog)
	for cogName, cogCfg := range cfg.Discord.Cogs {
		cog, ok := availableCogs[cogName]
		if !ok {
			return nil, fmt.Errorf("cog %s not found", cogName)
		}

		cog.Setup(ctx, cfg)
		for _, commandName := range cogCfg.Commands {
			if existingCog, ok := existingCommands[commandName]; ok {
				return nil, fmt.Errorf("duplicate command name %s in cogs %s and %s", commandName, existingCog, cog)
			}

			if err := cog.ActivateCommand(commandName); err != nil {
				return nil, fmt.Errorf("cog %s does not implement command %s", cogName, commandName)
			}

			existingCommands[commandName] = cog
		}

		cogs = append(cogs, cog)
	}

	for _, cog := range cogs {
		logger.Infof("Loaded cog %s with commands %s", cog, cog.ActiveCommands())
	}

	return cogs, nil
}
