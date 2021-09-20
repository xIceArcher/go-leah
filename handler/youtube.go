package handler

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/youtube"
	"go.uber.org/zap"
)

func YoutubeLiveStream(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, videoIDs []string) (err error) {
	api, err := youtube.NewAPI(cfg.Google)
	if err != nil {
		return err
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(videoIDs))

	for _, videoID := range videoIDs {
		if len(embeds) == 10 {
			zap.S().Warn("More than 10 embeds in one message")
			break
		}

		logger := zap.S().With(
			"videoID", videoID,
		)

		video, err := api.GetVideo(videoID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video info")
			continue
		}

		videoEmbed, err := video.GetEmbed(cfg, true)
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
		_, err = session.ChannelMessageSendEmbeds(msg.ChannelID, embeds)
	}
	return err
}
