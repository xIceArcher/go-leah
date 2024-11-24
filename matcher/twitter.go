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

	if !isDiscordMainEmbedCorrect(tweet, existingEmbeds) {
		s.SendEmbeds(tweet.GetEmbeds())
	}

	if tweet.HasVideos() && (len(existingEmbeds) == 0 || existingEmbeds[0].Video == nil) {
		for _, video := range tweet.Videos() {
			s.SendVideoURL(video.URL, s.Message.ID)
		}
	}
}

func isDiscordMainEmbedCorrect(tweet *twitter.Tweet, existingEmbeds discord.UpdatableMessageEmbeds) bool {
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

	// Discord can't embed tweets that are too long
	if len(tweet.Text) > len(existingEmbeds[0].Description) {
		return false
	}

	// Discord can't embed alt texts
	for _, media := range tweet.Medias {
		if media.AltText != "" {
			return false
		}
	}

	// Additional checks for tweets with photos
	if tweet.HasPhotos() {
		// The embed is missing the photo(s)
		if existingEmbeds[0].Image == nil {
			return false
		}

		// Discord can't embed more than one photo
		if len(tweet.Photos()) > 1 {
			return false
		}

		// If the first media is a video, then Discord will wrongly embed it as a photo, and will not embed the rest of the photos
		if tweet.Medias[0].Type == twitter.MediaTypeVideo {
			return false
		}

		// At this point, we know:
		// (1) The existing embed has a photo
		// (2) The tweet only has one photo (and potentially videos in some order)
		// (3) The first media of this tweet is a photo
		// (2) and (3) imply that the tweet's first media is the only photo
		// Combied with (1), this means that the tweet's only photo is correctly embedded
	}

	return true
}
