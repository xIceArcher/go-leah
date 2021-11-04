package youtube

import (
	"context"
	"errors"
	"sync"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
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

var ErrNotFound = errors.New("resource not found")

type API struct{}

var (
	api          *youtube.Service
	apiSetupOnce sync.Once
)

func NewAPI(cfg *config.GoogleConfig) (*API, error) {
	var err error
	apiSetupOnce.Do(func() {
		api, err = youtube.NewService(context.Background(), option.WithAPIKey(cfg.APIKey))
	})

	return &API{}, err
}

func (API) GetVideo(id string, parts []string) (*Video, error) {
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

	video.Channel, err = API{}.GetChannel(video.ChannelID, []string{PartSnippet})
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

func (API) GetChannel(id string, parts []string) (*Channel, error) {
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
