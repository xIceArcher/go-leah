package matcher

import (
	"context"
	"strings"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/instagram"
	"go.uber.org/zap"
)

type InstagramPostMatcher struct {
	GenericMatcher

	api *instagram.API
}

func NewInstagramPostMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	api, err := instagram.NewAPI(cfg.Instagram)
	if err != nil {
		return nil, err
	}

	return &InstagramPostMatcher{
		api: api,
	}, nil
}

func (m *InstagramPostMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	for _, shortcode := range matches {
		logger := s.Logger.With(
			zap.String("shortcode", shortcode),
		)

		post, err := m.api.GetPost(shortcode)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get post")
			continue
		}

		s.SendEmbeds(post.GetEmbeds())
		s.SendVideos(post.VideoURLs, shortcode)
	}
}

type InstagramStoryMatcher struct {
	GenericMatcher

	api *instagram.API
}

func NewInstagramStoryMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	api, err := instagram.NewAPI(cfg.Instagram)
	if err != nil {
		return nil, err
	}

	return &InstagramStoryMatcher{
		api: api,
	}, nil
}

func (m *InstagramStoryMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	for _, match := range matches {
		logger := s.Logger.With(
			zap.String("match", match),
		)

		username := match
		storyID := ""
		if strings.Contains(match, "/") {
			parts := strings.Split(match, "/")
			if len(parts) != 2 {
				logger.Error("Unknown match")
				continue
			}

			username, storyID = parts[0], parts[1]
		}

		story, err := m.api.GetStory(username, storyID)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get story")
			continue
		}

		s.DownloadImageAndSendEmbed(story.GetEmbed(), username)
		if story.IsVideo() {
			s.SendVideo(story.MediaURL, username)
		}
	}
}
