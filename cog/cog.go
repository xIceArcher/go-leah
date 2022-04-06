package cog

import (
	"context"
	"fmt"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"golang.org/x/exp/maps"
)

var (
	ErrIllegalAccess error = fmt.Errorf("illegal access")
)

type Cog interface {
	Handle(context.Context, *discord.MessageSession, string, []string)
	HasCommand(string) bool
	Commands() []string
	Stop()
}

type Constructor func(*config.Config, *discord.Session) (Cog, error)
type CommandFunc func(context.Context, *discord.MessageSession, []string)

type GenericCog struct {
	allCommands map[string]CommandFunc
}

func (c *GenericCog) Handle(ctx context.Context, s *discord.MessageSession, cmd string, args []string) {
	commandFunc, ok := c.allCommands[cmd]
	if ok {
		commandFunc(ctx, s, args)
	}
}

func (c *GenericCog) HasCommand(cmd string) bool {
	_, ok := c.allCommands[cmd]
	return ok
}

func (c *GenericCog) Commands() []string {
	return maps.Keys(c.allCommands)
}

func (c *GenericCog) Stop() {}
