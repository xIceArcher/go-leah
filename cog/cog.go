package cog

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var implementedCogs []Cog = []Cog{
	&AdminCog{},
	&TwitterCog{},
	&DownloadCog{},
	&TweetStalkCog{},
}

var (
	ErrNotImplemented          error = fmt.Errorf("not implemented")
	ErrInsufficientPermissions error = fmt.Errorf("insufficient permissions")
	ErrIllegalAccess           error = fmt.Errorf("illegal access")
)

type Cog interface {
	Setup(ctx context.Context, cfg *config.Config, wg *sync.WaitGroup) error
	Resume(ctx context.Context, session *discordgo.Session, logger *zap.SugaredLogger)
	Handle(ctx context.Context, command string, session *discordgo.Session, msg *discordgo.Message, args []string, logger *zap.SugaredLogger) error

	Commands() []Command
	WaitGroup() *sync.WaitGroup

	IsCommandActive(command string) bool
	ActiveCommands() []Command

	ActivateCommand(command string) error

	String() string
}

type DiscordBotCog struct {
	cog     Cog
	cogCfg  *config.DiscordCogConfig
	wg      *sync.WaitGroup
	adminID string

	activeCommandMap map[string]Command
}

func (c *DiscordBotCog) Setup(cog Cog, cfg *config.Config, wg *sync.WaitGroup) {
	c.cog = cog
	c.cogCfg = cfg.Discord.Cogs[cog.String()]
	c.wg = wg
	c.adminID = cfg.Discord.AdminID

	c.activeCommandMap = make(map[string]Command)

}

func (c *DiscordBotCog) Handle(ctx context.Context, command string, session *discordgo.Session, msg *discordgo.Message, args []string, logger *zap.SugaredLogger) error {
	if c.IsCommandActive(command) {
		if c.cogCfg.IsAdminOnly && msg.Author.ID != c.adminID {
			return ErrInsufficientPermissions
		}

		if len(c.cogCfg.ChannelIDs) > 0 && !slices.Contains(c.cogCfg.ChannelIDs, msg.ChannelID) {
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

func (c *DiscordBotCog) WaitGroup() *sync.WaitGroup {
	return c.wg
}

type Command interface {
	Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) error
	String() string
}

func SetupCogs(ctx context.Context, cfg *config.Config, wg *sync.WaitGroup, logger *zap.SugaredLogger) (cogs []Cog, err error) {
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

		cog.Setup(ctx, cfg, wg)
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
