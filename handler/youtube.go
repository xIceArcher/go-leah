package handler

import (
	"context"
	"errors"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/youtube"
	"go.uber.org/zap"
)

type YoutubeLiveStreamHandler struct {
	api *youtube.API

	RegexManager
}

func (YoutubeLiveStreamHandler) Name() string {
	return "youtubeLiveStream"
}

func (h *YoutubeLiveStreamHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp) (err error) {
	h.Regexes = regexes
	h.api, err = youtube.NewAPI(cfg.Google)
	return err
}

func (h *YoutubeLiveStreamHandler) Handle(session *discordgo.Session, channelID string, msg string, logger *zap.SugaredLogger) (videoIDs []string, err error) {
	videoIDs = h.Match(msg)
	embeds := make([]*discordgo.MessageEmbed, 0, len(videoIDs))

	for _, videoID := range videoIDs {
		if len(embeds) == 10 {
			logger.Warn("More than 10 embeds in one message")
			break
		}

		logger := logger.With(
			zap.String("videoID", videoID),
		)

		video, err := h.api.GetVideo(videoID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video info")
			continue
		}

		videoEmbed, err := video.GetEmbed(true)
		if errors.Is(err, youtube.ErrNotLivestream) {
			logger.Info("Not a livestream")
			continue
		}
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video embed")
		}

		embeds = append(embeds, videoEmbed)
	}

	if len(embeds) > 0 {
		_, err = session.ChannelMessageSendEmbeds(channelID, embeds)
	}
	return videoIDs, err
}
