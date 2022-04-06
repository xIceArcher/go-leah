package cog

import (
	"context"
	"fmt"
	"strings"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
)

type AdminCog struct {
	GenericCog
}

func NewAdminCog(cfg *config.Config, s *discord.Session) (Cog, error) {
	c := &AdminCog{}

	c.allCommands = map[string]CommandFunc{
		"servers": c.Servers,
	}

	return c, nil
}

func (c *AdminCog) Servers(ctx context.Context, s *discord.MessageSession, args []string) {
	guilds := make([]string, 0)
	for _, guild := range s.State.Guilds {
		guilds = append(guilds, guild.Name)
	}

	s.SendMessage(fmt.Sprintf("Active servers: %s", strings.Join(guilds, ", ")))
}
