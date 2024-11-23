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
		for _, video := range tweet.Videos() {
			s.SendVideoURL(video.URL, s.Message.ID)
		}
		return
	}

	if !isDiscordEmbedCorrect(tweet, existingEmbeds) {
		s.SendEmbeds(tweet.GetEmbeds())
	}

	if tweet.HasVideos() && existingEmbeds[0].Video == nil {
		for _, video := range tweet.Videos() {
			s.SendVideoURL(video.URL, s.Message.ID)
		}
	}
}

func isDiscordEmbedCorrect(tweet *twitter.Tweet, existingEmbeds discord.UpdatableMessageEmbeds) bool {
	// Discord can't embed polls
	if tweet.Poll != nil {
		return false
	}

	// Embed is missing
	if len(existingEmbeds) == 0 {
		return false
	}

	// Embed exists but text is missing
	if existingEmbeds[0].Description == "" && tweet.Text != "" {
		return false
	}

	// Additional checks for tweets with media
	if len(tweet.Medias) != 0 {
		// If the first media is a video, then Discord will wrongly embed it as a photo
		if tweet.Medias[0].Type == twitter.MediaTypeVideo {
			return false
		}

		// Discord can't embed more than one photo
		if len(tweet.Photos()) > 1 {
			return false
		}

		// Discord can't embed alt texts
		if tweet.Photos()[0].AltText != "" {
			return false
		}

		// At this point, the first media of this tweet is an image
		// If the existing embed doesn't have an image, then it is wrong
		if existingEmbeds[0].Image == nil {
			return false
		}
	}

	return true
}
