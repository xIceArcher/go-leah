package handler

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/tiktok"
	"github.com/xIceArcher/go-leah/utils"
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

func (h *TiktokVideoHandler) Handle(ctx context.Context, session *discordgo.Session, guildID string, channelID string, msg string, logger *zap.SugaredLogger) (ids []string, err error) {
	ids = h.Match(msg)

	for _, id := range ids {
		logger := logger.With(
			zap.String("id", id),
		)

		if _, err := strconv.Atoi(id); err != nil {
			// This is a short link, expand it
			expandedURL, err := utils.ExpandURL(fmt.Sprintf("https://vt.tiktok.com/%s", id))
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to expand URL")
				continue
			}

			newMatches := h.Match(expandedURL)
			if len(newMatches) > 0 {
				id = h.Match(expandedURL)[0]
			} else {
				logger.Info("Not video URL")
				continue
			}
		}

		video, err := h.api.GetVideo(id)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video")
			continue
		}

		maxMessageBytes := utils.GetDiscordMessageMaxBytes(discordgo.PremiumTierNone)

		guild, err := session.Guild(guildID)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get guild")
		} else {
			maxMessageBytes = utils.GetDiscordMessageMaxBytes(guild.PremiumTier)
		}

		messages := video.GetDiscordMessages(maxMessageBytes)

		for _, message := range messages {
			if _, err := session.ChannelMessageSendComplex(channelID, message); err != nil {
				logger.With(zap.Error(err)).Error("Failed to send message")
				continue
			}
		}
	}

	return ids, nil
}
