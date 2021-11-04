package handler

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"github.com/xIceArcher/go-leah/youtube"
	"go.uber.org/zap"
)

type YoutubeLiveStreamHandler struct {
	api *youtube.API

	RegexManager
}

func (YoutubeLiveStreamHandler) String() string {
	return "youtubeLiveStream"
}

func (h *YoutubeLiveStreamHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp) (err error) {
	h.Regexes = regexes
	h.api, err = youtube.NewAPI(cfg.Google)
	return err
}

func (h *YoutubeLiveStreamHandler) Handle(ctx context.Context, session *discordgo.Session, channelID string, msg string, logger *zap.SugaredLogger) (videoIDs []string, err error) {
	videoIDs = h.Match(msg)

	videos := make([]*youtube.Video, 0, len(videoIDs))
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
			if err == youtube.ErrNotFound {
				logger.Info("Video not found")
			} else {
				logger.With(zap.Error(err)).Error("Get video info")
			}

			continue
		}

		embed, err := video.GetEmbed(true)
		if errors.Is(err, youtube.ErrNotLiveStream) {
			logger.Info("Not a livestream")
			continue
		}
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video embed")
		}

		videos = append(videos, video)
		embeds = append(embeds, embed)
	}

	updatableEmbeds := utils.NewEmbeds(session, embeds)
	if err := updatableEmbeds.Send(channelID); err != nil {
		return nil, err
	}

	for i := 0; i < len(embeds); i++ {
		logger := logger.With(
			zap.String("videoID", videos[i].ID),
		)
		go h.watchVideo(ctx, videos[i], updatableEmbeds.Embeds[i], logger)
	}

	return videoIDs, nil
}

func (h *YoutubeLiveStreamHandler) watchVideo(ctx context.Context, video *youtube.Video, updatableEmbed *utils.UpdatableEmbed, logger *zap.SugaredLogger) {
	var err error
	for {
		var nextTickDuration time.Duration
		if time.Until(video.LiveStreamingDetails.ScheduledStartTime) > time.Hour {
			// If there's more than an hour until the video starts, refresh 2 minutes after the 1 hour before mark
			nextTickDuration = time.Until(video.LiveStreamingDetails.ScheduledStartTime.Add(-58 * time.Minute))
		} else if time.Now().Before(video.LiveStreamingDetails.ScheduledStartTime) {
			// Else if the video has not started yet, refresh 2 minutes after the scheduled start time
			nextTickDuration = time.Until(video.LiveStreamingDetails.ScheduledStartTime.Add(2 * time.Minute))
		} else if video.Duration != time.Duration(0) {
			// Else if this is a premiere, refresh 3 minutes after the video ends
			nextTickDuration = video.Duration + 3*time.Minute
		} else {
			// Otherwise refresh every 5 minutes
			nextTickDuration = 5 * time.Minute
		}

		ticker := time.NewTicker(nextTickDuration)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			video, err = h.api.GetVideo(video.ID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to get video")
				break
			}

			embed, err := video.GetEmbed(false)
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to get embed")
				break
			}

			updatableEmbed.MessageEmbed = embed
			if err := updatableEmbed.Update(); err != nil {
				logger.With(zap.Error(err)).Error("Failed to update embed")
				break
			}

			if video.IsDone {
				logger.Info("Video done")
				return
			}
		}
	}
}
