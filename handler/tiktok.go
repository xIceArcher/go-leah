package handler

import (
	"context"
	"regexp"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/tiktok"
	"go.uber.org/zap"
)

type TiktokVideoHandler struct {
	api *tiktok.API

	RegexManager
	unimplementedHandler
}

func (TiktokVideoHandler) String() string {
	return "tiktokVideo"
}

func (h *TiktokVideoHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp, wg *sync.WaitGroup) (err error) {
	h.Regexes = regexes
	h.api, err = tiktok.NewAPI(cfg.Tiktok)
	return err
}

func (h *TiktokVideoHandler) Handle(ctx context.Context, session *discordgo.Session, channelID string, msg string, logger *zap.SugaredLogger) (ids []string, err error) {
	ids = h.Match(msg)

	for _, id := range ids {
		logger := logger.With(
			zap.String("id", id),
		)

		video, err := h.api.GetVideo(id)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video")
			continue
		}

		messages := video.GetDiscordMessages()

		for _, message := range messages {
			if _, err := session.ChannelMessageSendComplex(channelID, message); err != nil {
				logger.With(zap.Error(err)).Error("Failed to send message")
				continue
			}
		}
	}

	return ids, nil
}
