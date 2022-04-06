package handler

import (
	"context"

	"github.com/xIceArcher/go-leah/discord"
)

type MessageHandler interface {
	Handle(context.Context, *discord.MessageSession) bool
	Stop()
}

type GenericHandler struct{}

func (h *GenericHandler) Handle(context.Context, *discord.MessageSession) bool { return false }
func (h *GenericHandler) Stop()                                                {}

type MessageHandlers []MessageHandler

func (hs MessageHandlers) HandleOne(ctx context.Context, s *discord.MessageSession) {
	for _, handler := range hs {
		if handler.Handle(ctx, s) {
			return
		}
	}
}

func (hs MessageHandlers) HandleAll(ctx context.Context, s *discord.MessageSession) {
	for _, handler := range hs {
		handler.Handle(ctx, s)
	}
}
