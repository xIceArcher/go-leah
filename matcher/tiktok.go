package matcher

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/tiktok"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

type TiktokVideoMatcher struct {
	GenericMatcher

	api *tiktok.API
}

func NewTiktokVideoMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	api, err := tiktok.NewAPI()
	if err != nil {
		return nil, err
	}

	return &TiktokVideoMatcher{
		api: api,
	}, nil
}

func (h *TiktokVideoMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	for _, id := range matches {
		logger := s.Logger.With(
			zap.String("id", id),
		)

		if _, err := strconv.Atoi(id); err != nil {
			// This is a short link, expand it
			expandedURL, err := utils.ExpandURL(fmt.Sprintf("https://vt.tiktok.com/%s", id))
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to expand URL")
				continue
			}

			u, err := url.Parse(expandedURL)
			if err != nil {
				logger.With(zap.Error(err)).Error("Failed to parse URL")
				continue
			}

			_, id = path.Split(u.Path)
		}

		video, err := h.api.GetVideo(id)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video")
			continue
		}

		s.SendEmbed(video.GetEmbed())
		s.SendVideo(video.Video, video.ID)
	}
}
