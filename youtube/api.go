package youtube

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/xIceArcher/go-leah/config"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	PartLiveStreamingDetails = "liveStreamingDetails"
	PartContentDetails       = "contentDetails"
	PartSnippet              = "snippet"
)

type API struct{}

var (
	api          *youtube.Service
	apiSetupOnce sync.Once
)

func NewAPI(cfg *config.GoogleConfig) (API, error) {
	var err error
	apiSetupOnce.Do(func() {
		api, err = youtube.NewService(context.Background(), option.WithAPIKey(cfg.APIKey))
	})

	return API{}, err
}

func (API) GetVideoInfo(id string, parts []string) (*youtube.Video, error) {
	videosInfo, err := api.Videos.List(parts).Id(id).Do()
	if err != nil {
		return nil, err
	}

	if len(videosInfo.Items) == 0 {
		return nil, fmt.Errorf("video %s not found", id)
	}

	return videosInfo.Items[0], nil
}

func (API) GetChannelInfo(id string, parts []string) (*youtube.Channel, error) {
	channelsInfo, err := api.Channels.List(parts).Id(id).Do()
	if err != nil {
		return nil, err
	}

	if len(channelsInfo.Items) == 0 {
		return nil, fmt.Errorf("channel %s not found", id)
	}

	return channelsInfo.Items[0], nil
}

func (API) GetBestThumbnail(details *youtube.ThumbnailDetails) (*youtube.Thumbnail, error) {
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

func (API) GetVideoURL(id string) string {
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", id)
}

func (API) GetChannelURL(id string) string {
	return fmt.Sprintf("https://www.youtube.com/channel/%s", id)
}
