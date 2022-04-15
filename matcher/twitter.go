package matcher

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/twitter"
	"go.uber.org/zap"
)

type TwitterSpaceMatcher struct {
	GenericMatcher

	api twitter.API
}

func NewTwitterSpaceMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	return &TwitterSpaceMatcher{
		api: twitter.NewBaseAPI(cfg.Twitter),
	}, nil
}

func (m *TwitterSpaceMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	embeds := make([]*discordgo.MessageEmbed, 0, len(matches))

	for _, spaceID := range matches {
		logger := s.Logger.With(
			zap.String("spaceID", spaceID),
		)

		space, err := m.api.GetSpace(spaceID)
		if err != nil {
			logger.With(zap.Error(err)).Info("Failed to get space ID")
			continue
		}

		embeds = append(embeds, space.GetEmbed())
	}

	s.SendEmbeds(embeds)
}

type TwitterPostMatcher struct {
	GenericMatcher

	api twitter.API
}

func NewTwitterPostMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	return &TwitterPostMatcher{
		api: twitter.NewBaseAPI(cfg.Twitter),
	}, nil
}

func (m *TwitterPostMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	time.Sleep(5 * time.Second)

	existingEmbeds, err := s.GetMessageEmbeds()
	if err != nil {
		s.Logger.With(zap.Error(err)).Warn("Failed to get message embeds")
		return
	}

	if len(existingEmbeds) > 0 {
		return
	}

	for _, tweetID := range matches {
		logger := s.Logger.With(
			zap.String("tweetID", tweetID),
		)

		tweet, err := m.api.GetTweet(tweetID)
		if err != nil {
			logger.With(zap.Error(err)).Info("Failed to get tweet ID")
			continue
		}

		s.SendEmbeds(tweet.GetEmbeds())
	}
}
