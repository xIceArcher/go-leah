package matcher

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/utils"
	"github.com/xIceArcher/go-leah/youtube"
	"go.uber.org/zap"
)

const (
	CacheKeyYoutubeLiveStreamPrefix = "go-leah/youtubeLiveStream/"
	CacheKeyYoutubeLiveStreamFormat = CacheKeyYoutubeLiveStreamPrefix + "%s/%s/%v"
)

type YoutubeLiveStreamMatcher struct {
	GenericMatcher

	api   youtube.API
	cache cache.Cache

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewYoutubeLiveStreamMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	c, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		return nil, err
	}

	a, err := youtube.NewCachedAPI(cfg.Google, c, s.Logger)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	matcher := &YoutubeLiveStreamMatcher{
		api:   a,
		cache: c,

		ctx:    ctx,
		cancel: cancel,
	}

	matcher.resumeOldTasks(s)
	return matcher, nil
}

func (m *YoutubeLiveStreamMatcher) resumeOldTasks(s *discord.Session) {
	oldTasks, err := m.cache.GetByPrefix(m.ctx, CacheKeyYoutubeLiveStreamPrefix)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to fetch old tasks")
	}

	for taskKey, taskValue := range oldTasks {
		videoID := fmt.Sprintf("%v", taskValue)

		video, err := m.api.GetVideo(m.ctx, videoID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
		if err != nil {
			s.Logger.With(zap.Error(err), zap.String("videoID", videoID)).Warn("Failed to get video")
			continue
		}

		key := strings.TrimPrefix(taskKey, CacheKeyYoutubeLiveStreamPrefix)
		keySplit := strings.Split(key, "/")
		if len(keySplit) != 3 {
			s.Logger.With(zap.String("key", key)).Warn("Unknown key")
			continue
		}

		channelID, messageID, idxStr := keySplit[0], keySplit[1], keySplit[2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			s.Logger.With(zap.Error(err), zap.String("key", key)).Warn("Failed to parse key")
			continue
		}

		embeds, err := s.GetMessageEmbeds(channelID, messageID)
		if err != nil {
			s.Logger.With(zap.Error(err), zap.String("channelID", channelID), zap.String("messageID", messageID)).Warn("Failed to get message")
			continue
		}

		if idx >= len(embeds) {
			s.Logger.With(zap.Int("expectedIdx", idx), zap.Int("numEmbeds", len(embeds))).Warn("Failed to get embed")
			continue
		}

		go m.watchVideoTask(taskKey, video, embeds[idx], s.Logger)

		if err := m.cache.Clear(m.ctx, taskKey); err != nil {
			s.Logger.With(zap.Error(err)).Error("Failed to clear cache key")
		}
	}

}

func (m *YoutubeLiveStreamMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	videos := make([]*youtube.Video, 0, len(matches))
	embeds := make([]*discordgo.MessageEmbed, 0, len(matches))

	for _, videoID := range matches {
		logger := s.Logger.With(
			zap.String("videoID", videoID),
		)

		video, err := m.api.GetVideo(ctx, videoID, []string{
			youtube.PartLiveStreamingDetails,
			youtube.PartContentDetails,
			youtube.PartSnippet,
		})
		if errors.Is(err, youtube.ErrNotFound) {
			logger.Info("Video not found")
			continue
		} else if err != nil {
			logger.With(zap.Error(err)).Error("Get video info")
			continue
		}

		if !video.IsActiveLivestream() {
			logger.Info("Not active livestream")
			continue
		}

		videos = append(videos, video)
		embeds = append(embeds, video.GetEmbed())
	}

	updatableEmbeds, err := s.SendEmbeds(embeds)
	if err == nil {
		for i, embed := range updatableEmbeds {
			cacheKey := fmt.Sprintf(CacheKeyYoutubeLiveStreamFormat, embed.ChannelID, embed.Message.ID, i)
			go m.watchVideoTask(cacheKey, videos[i], updatableEmbeds[i], s.Logger)
		}
	}
}

func (m *YoutubeLiveStreamMatcher) watchVideoTask(cacheKey string, video *youtube.Video, embed *discord.UpdatableMessageEmbed, logger *zap.SugaredLogger) {
	m.wg.Add(1)
	defer m.wg.Done()

	logger = logger.With(zap.String("videoID", video.ID))

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

		select {
		case <-m.ctx.Done():
			// Cannot use ctx here since it has already been cancelled
			err := m.cache.Set(context.Background(), cacheKey, video.ID)
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to write to cache")
			}
			return
		case <-time.After(time.Until(nextTickTime)):
			startTime := video.LiveStreamingDetails.ActualStartTime

			var err error
			video, err = m.api.GetVideo(m.ctx, video.ID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
			if err != nil {
				// Assume the video ended and became unlisted
				embed.Fields = []*discordgo.MessageEmbedField{
					{
						Name:   "Ended",
						Value:  "~" + utils.FormatDiscordRelativeTime(time.Now()),
						Inline: true,
					},
					{
						Name:   "Duration",
						Value:  "~" + utils.FormatDurationSimple(time.Now().Sub(startTime)),
						Inline: true,
					},
				}

				embed.Color = utils.ParseHexColor(consts.ColorNone)
			} else {
				embed.MessageEmbed = video.GetEmbed()
			}

			if err := embed.Update(); err != nil {
				logger.With(zap.Error(err)).Error("Failed to update embed")
				return
			}

			if video == nil || video.IsDone {
				logger.Info("Video done")
				return
			}
		}
	}
}

func (m *YoutubeLiveStreamMatcher) Stop() {
	m.cancel()
	m.wg.Wait()
}
