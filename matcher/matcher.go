package matcher

import (
	"context"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
)

type Matcher interface {
	Handle(context.Context, *discord.MessageSession, []string)
	Stop()
}

type Constructor func(cfg *config.Config, s *discord.Session) (Matcher, error)

type GenericMatcher struct{}

func (m *GenericMatcher) Handle(context.Context, *discord.MessageSession, []string) {}

func (m *GenericMatcher) Stop() {}
