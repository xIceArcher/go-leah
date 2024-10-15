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

		embeds := post.GetEmbeds()
		if len(embeds) > 10 {
			// We send 8 embeds per message because Discord tiles 4 embeds into a single frame
			// And each message can only have a maximum of 10 embeds
			for start := 0; start < len(embeds); start += 8 {
				end := min(start+8, len(embeds))
				s.SendEmbeds(post.GetEmbeds()[start:end])
			}
		} else {
			s.SendEmbeds(post.GetEmbeds())
		}

		s.SendVideoURLs(post.VideoURLs, shortcode)
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

		var (
			username string
			story    *instagram.Story
			err      error
		)
		if strings.Contains(match, "/") {
			parts := strings.Split(match, "/")
			if len(parts) != 2 {
				logger.Error("Unknown match")
				continue
			}

			username = parts[0]
			storyID := parts[1]

			story, err = m.api.GetStory(username, storyID)
		} else {
			username = match
			story, err = m.api.GetLatestStory(username)
		}

		if err != nil {
			logger.With(zap.Error(err)).Error("Get story")
			continue
		}

		s.DownloadImageAndSendEmbed(story.GetEmbed(), username)
		if story.IsVideo() {
			s.SendVideoURL(story.MediaURL, username)
		}
	}
}
