package matcher

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
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

type InstagramShareLinkMatcher struct {
	GenericMatcher

	postRegexes []*regexp.Regexp
	postMatcher Matcher
}

func NewInstagramShareLinkMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	postMatcher, err := NewInstagramPostMatcher(cfg, s)
	if err != nil {
		return nil, err
	}

	postHandlerConfig, ok := cfg.Discord.Handlers["instagramPost"]
	if !ok {
		return nil, fmt.Errorf("instagramPost handler config not found")
	}

	postRegexes := make([]*regexp.Regexp, 0, len(postHandlerConfig.Regexes))

	for _, regexStr := range postHandlerConfig.Regexes {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, err
		}

		postRegexes = append(postRegexes, regex)
	}

	return &InstagramShareLinkMatcher{
		postRegexes: postRegexes,
		postMatcher: postMatcher,
	}, nil
}

func (m *InstagramShareLinkMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	for _, match := range matches {
		resp, err := http.Head(match)
		if err != nil {
			continue
		}

		url := resp.Request.URL.String()

		postMatches := make([]string, 0)
		for _, regex := range m.postRegexes {
			currMatches := regex.FindAllStringSubmatch(url, -1)
			for _, match := range currMatches {
				if len(match) > 1 {
					// match[1] is the subgroup
					postMatches = append(postMatches, match[1])
				} else {
					// match[0] is the complete match
					postMatches = append(postMatches, match[0])
				}
			}
		}

		m.postMatcher.Handle(ctx, s, postMatches)
	}
}
