package matcher

import (
	"context"
	"time"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/twitter"
	"go.uber.org/zap"
)

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

	// TODO: Find some way to match tweets to existing embeds
	if len(matches) > 1 && len(existingEmbeds) > 0 {
		return
	}

	tweetID := matches[0]

	logger := s.Logger.With(
		zap.String("tweetID", tweetID),
	)

	tweet, err := m.api.GetTweet(tweetID)
	if err != nil {
		logger.With(zap.Error(err)).Info("Failed to get tweet ID")
		return
	}

	// Whole embed is missing or corrupted
	if len(existingEmbeds) == 0 || (existingEmbeds[0].Description == "" && tweet.Text != "") {
		s.SendEmbeds(tweet.GetEmbeds())
		for _, videoURL := range tweet.VideoURLs {
			s.SendVideoURL(videoURL, s.Message.ID)
		}
		return
	}

	if (tweet.HasPhotos() && (existingEmbeds[0].Image == nil || tweet.Photos[0].AltText != "")) || len(tweet.Photos) > 1 || tweet.Poll != nil {
		s.SendEmbeds(tweet.GetEmbeds())
	}

	if tweet.HasVideos() && existingEmbeds[0].Video == nil {
		for _, videoURL := range tweet.VideoURLs {
			s.SendVideoURL(videoURL, s.Message.ID)
		}
	}
}
