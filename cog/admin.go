package cog

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type AdminCog struct {
	DiscordBotCog
}

func (AdminCog) String() string {
	return "admin"
}

func (c *AdminCog) Setup(ctx context.Context, cfg *config.Config) error {
	c.DiscordBotCog.Setup(c, cfg)
	return nil
}

func (AdminCog) Commands() []Command {
	return []Command{
		&ServersCommand{},
		&RestartCommand{},
	}
}

type ServersCommand struct{}

func (ServersCommand) String() string {
	return "servers"
}

func (ServersCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) error {
	guilds := make([]string, 0)
	for _, guild := range session.State.Guilds {
		guilds = append(guilds, guild.Name)
	}

	_, err := session.ChannelMessageSend(channelID, fmt.Sprintf("Active servers: %s", strings.Join(guilds, ", ")))
	return err
}

type RestartCommand struct{}

func (RestartCommand) String() string {
	return "restart"
}

func (RestartCommand) Handle(ctx context.Context, session *discordgo.Session, channelID string, args []string, logger *zap.SugaredLogger) error {
	// Dummy command, restart is handled earlier in the call stack
	return nil
}
