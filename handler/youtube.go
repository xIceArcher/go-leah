package handler

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"github.com/xIceArcher/go-leah/youtube"
	"go.uber.org/zap"
)

const (
	CacheKeyYoutubeLiveStreamPrefix = "go-leah/youtubeLiveStream/"
	CacheKeyYoutubeLiveStreamFormat = CacheKeyYoutubeLiveStreamPrefix + "%s/%s/%v"
)

type YoutubeLiveStreamHandler struct {
	api   *youtube.API
	cache *cache.Cache
	wg    *sync.WaitGroup

	RegexManager
}

func (YoutubeLiveStreamHandler) String() string {
	return "youtubeLiveStream"
}

func (h *YoutubeLiveStreamHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp, wg *sync.WaitGroup) (err error) {
	h.Regexes = regexes
	h.wg = wg

	h.api, err = youtube.NewAPI(cfg.Google)
	if err != nil {
		return err
	}

	h.cache, err = cache.NewCache(cfg.Redis)
	return err
}

func (h *YoutubeLiveStreamHandler) Resume(ctx context.Context, session *discordgo.Session, logger *zap.SugaredLogger) {
	oldTasks, err := h.cache.GetByPrefix(ctx, CacheKeyYoutubeLiveStreamPrefix)
	if err != nil {
		logger.With(zap.Error(err)).Error("Failed to fetch old tasks")
	}

	for taskKey, taskValue := range oldTasks {
		key := strings.TrimPrefix(taskKey, CacheKeyYoutubeLiveStreamPrefix)
		keySplit := strings.Split(key, "/")
		if len(keySplit) != 3 {
			logger.With(zap.String("key", key)).Warn("Unknown key")
			continue
		}

		channelID, messageID, idxStr := keySplit[0], keySplit[1], keySplit[2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			logger.With(zap.Error(err), zap.String("key", key)).Warn("Failed to parse key")
			continue
		}

		videoID := fmt.Sprintf("%v", taskValue)

		video, err := h.api.GetVideo(videoID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
		if err != nil {
			logger.With(zap.Error(err), zap.String("videoID", videoID)).Warnf("Failed to get video")
			continue
		}

		embed, err := utils.LoadEmbed(session, channelID, messageID, idx)
		if err != nil {
			logger.With(zap.Error(err), zap.String("channelID", channelID), zap.String("messageID", messageID)).Warnf("Failed to get message")
			continue
		}

		logger := logger.With(zap.String("videoID", videoID))
		go h.watchVideo(ctx, video, embed, logger)

		if err := h.cache.Clear(ctx, taskKey); err != nil {
			logger.With(zap.Error(err)).Error("Failed to clear cache key")
		}
	}
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
	h.wg.Add(1)
	defer h.wg.Done()

	cacheKey := fmt.Sprintf(CacheKeyYoutubeLiveStreamFormat, updatableEmbed.ChannelID, updatableEmbed.MessageID, updatableEmbed.Idx)

	var err error
	for {
		var nextTickTime time.Time
		if video.IsDone {
			// If the video is already done, immediately update
			nextTickTime = time.Now()
		} else if time.Until(video.LiveStreamingDetails.ScheduledStartTime) > time.Hour {
			// If there's more than an hour until the video starts, refresh 2 minutes after the 1 hour before mark
			nextTickTime = video.LiveStreamingDetails.ScheduledStartTime.Add(-58 * time.Minute)
		} else if time.Now().Before(video.LiveStreamingDetails.ScheduledStartTime) {
			// Else if the video has not started yet, refresh 2 minutes after the scheduled start time
			nextTickTime = video.LiveStreamingDetails.ScheduledStartTime.Add(2 * time.Minute)
		} else if video.Duration != time.Duration(0) {
			// Else if this is a premiere, refresh 3 minutes after the video ends
			nextTickTime = video.LiveStreamingDetails.ScheduledStartTime.Add(video.Duration).Add(3 * time.Minute)
		} else {
			// Otherwise refresh every 5 minutes
			nextTickTime = time.Now().Add(5 * time.Minute)
		}

		ticker := time.NewTicker(time.Until(nextTickTime))

		select {
		case <-ctx.Done():
			// Cannot use ctx here since it has already been cancelled
			err = h.cache.Set(context.Background(), cacheKey, video.ID)
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to write to cache")
			}
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
