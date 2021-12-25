package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	PartLiveStreamingDetails = "liveStreamingDetails"
	PartContentDetails       = "contentDetails"
	PartSnippet              = "snippet"

	LiveBroadcastContentNone     = "none"
	LiveBroadcastContentUpcoming = "upcoming"
	LiveBroadcastContentLive     = "live"
	LiveBroadcastContentDone     = "done"
)

const (
	CacheKeyYoutubeAPIFormat = "go-leah/youtubeAPI/%s-%s"
)

var ErrNotFound = errors.New("resource not found")

type API interface {
	GetVideo(ctx context.Context, id string, parts []string) (*Video, error)
	GetChannel(ctx context.Context, id string, parts []string) (*Channel, error)
}

type CachedYoutubeAPI struct {
	*YoutubeAPI

	cache  cache.Cache
	logger *zap.SugaredLogger
}

func NewCachedAPI(cfg *config.GoogleConfig, c cache.Cache, logger *zap.SugaredLogger) (API, error) {
	if _, err := NewAPI(cfg); err != nil {
		return nil, err
	}

	return &CachedYoutubeAPI{
		YoutubeAPI: &YoutubeAPI{},

		logger: logger,
		cache:  c,
	}, nil
}

func (a *CachedYoutubeAPI) GetVideo(ctx context.Context, id string, parts []string) (*Video, error) {
	cacheKey := fmt.Sprintf(CacheKeyYoutubeAPIFormat, id, strings.Join(parts, ","))
	logger := a.logger.With(zap.String("cacheKey", cacheKey))

	if video, err := func() (video *Video, err error) {
		val, err := a.cache.Get(ctx, cacheKey)
		if err != nil {
			return nil, err
		}

		valStr, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("unknown cache return type %T", val)
		}

		video = &Video{}
		err = json.Unmarshal([]byte(valStr), video)
		return
	}(); err == nil {
		return video, nil
	}

	video, err := a.YoutubeAPI.GetVideo(ctx, id, parts)
	if err != nil {
		return nil, err
	}

	videoBytes, err := json.Marshal(video)
	if err != nil {
		// This error only affects caching, ignore and return the result
		logger.With(zap.Error(err)).Warn("Failed to marshal video")
		return video, nil
	}

	if err := a.cache.SetWithExpiry(ctx, cacheKey, videoBytes, 4*time.Minute); err != nil {
		logger.With(zap.Error(err)).Warn("Failed to set cache")
	}

	return video, nil
}

type YoutubeAPI struct{}

var (
	api          *youtube.Service
	apiSetupOnce sync.Once
)

func NewAPI(cfg *config.GoogleConfig) (API, error) {
	var err error
	apiSetupOnce.Do(func() {
		api, err = youtube.NewService(context.Background(), option.WithAPIKey(cfg.APIKey))
	})

	return &YoutubeAPI{}, err
}

func (YoutubeAPI) GetVideo(ctx context.Context, id string, parts []string) (*Video, error) {
	videosInfo, err := api.Videos.List(parts).Id(id).Do()
	if err != nil {
		return nil, err
	}

	if len(videosInfo.Items) == 0 {
		return nil, ErrNotFound
	}

	videoInfo := videosInfo.Items[0]
	video := &Video{
		ID:        id,
		Title:     videoInfo.Snippet.Title,
		ChannelID: videoInfo.Snippet.ChannelId,
	}

	video.Channel, err = YoutubeAPI{}.GetChannel(ctx, video.ChannelID, []string{PartSnippet})
	if err != nil {
		return nil, err
	}

	if duration, ok := utils.ParseISODuration(videoInfo.ContentDetails.Duration); ok {
		video.Duration = duration
	}

	if thumbnail, err := getBestThumbnail(videoInfo.Snippet.Thumbnails); err == nil {
		video.ThumbnailURL = thumbnail.Url
	}

	video.IsDone = videoInfo.Snippet.LiveBroadcastContent == LiveBroadcastContentNone || videoInfo.Snippet.LiveBroadcastContent == LiveBroadcastContentDone

	if videoInfo.LiveStreamingDetails != nil {
		video.LiveStreamingDetails = &LiveStreamingDetails{
			ConcurrentViewers: videoInfo.LiveStreamingDetails.ConcurrentViewers,
		}

		if actualStartTime, ok := utils.ParseISOTime(videoInfo.LiveStreamingDetails.ActualStartTime); ok {
			video.LiveStreamingDetails.ActualStartTime = actualStartTime
		}

		if scheduledStartTime, ok := utils.ParseISOTime(videoInfo.LiveStreamingDetails.ScheduledStartTime); ok {
			video.LiveStreamingDetails.ScheduledStartTime = scheduledStartTime
		}

		if actualEndTime, ok := utils.ParseISOTime(videoInfo.LiveStreamingDetails.ActualEndTime); ok {
			video.LiveStreamingDetails.ActualEndTime = actualEndTime
		}
	}

	return video, nil
}

func (YoutubeAPI) GetChannel(ctx context.Context, id string, parts []string) (*Channel, error) {
	channelsInfo, err := api.Channels.List(parts).Id(id).Do()
	if err != nil {
		return nil, err
	}

	if len(channelsInfo.Items) == 0 {
		return nil, ErrNotFound
	}

	channelInfo := channelsInfo.Items[0]
	channel := &Channel{
		ID:    id,
		Title: channelInfo.Snippet.Title,
	}

	if thumbnail, err := getBestThumbnail(channelInfo.Snippet.Thumbnails); err == nil {
		channel.ThumbnailURL = thumbnail.Url
	}

	return channel, nil
}

func getBestThumbnail(details *youtube.ThumbnailDetails) (*youtube.Thumbnail, error) {
	thumbnails := []*youtube.Thumbnail{
		details.Default, details.High, details.Maxres, details.Medium, details.Standard,
	}

	currMaxPixels := int64(0)
	currMaxIdx := -1
	for i, thumbnail := range thumbnails {
		if thumbnail == nil {
			continue
		}

		currPixels := thumbnail.Height * thumbnail.Width
		if currPixels > currMaxPixels {
			currMaxPixels = currPixels
			currMaxIdx = i
		}
	}

	if currMaxIdx == -1 {
		return nil, errors.New("no valid thumbnails")
	}

	return thumbnails[currMaxIdx], nil
}
