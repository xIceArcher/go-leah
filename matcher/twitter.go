package matcher

import (
	"context"
	"net/http"
	"slices"
	"strings"
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
		api: twitter.NewBaseAPI(),
	}, nil
}

func (m *TwitterPostMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	tweets := make([]*twitter.Tweet, 0, len(matches))
	for _, tweetID := range matches {
		logger := s.Logger.With(zap.String("tweetID", tweetID))

		tweet, err := m.api.GetTweet(tweetID)
		if err != nil {
			logger.With(zap.Error(err)).Info("Failed to get tweet ID")
			continue
		}

		tweets = append(tweets, tweet)
	}

	if len(tweets) == 0 {
		return
	}

	m.handleTweetMainEmbeds(ctx, s, tweets)

	for _, tweet := range tweets {
		if !tweet.HasVideos() {
			continue
		}

		for _, video := range tweet.Videos() {
			if video.Type == twitter.MediaTypeGIF && strings.HasSuffix(video.URL, ".mp4") {
				s.SendMP4URLAsGIF(video.URL, s.Message.ID)
			} else {
				s.SendVideoURL(video.URL, s.Message.ID)
			}
		}
	}
}

func (m *TwitterPostMatcher) handleTweetMainEmbeds(_ context.Context, s *discord.MessageSession, tweets []*twitter.Tweet) {
	var existingEmbeds discord.UpdatableMessageEmbeds
	if slices.ContainsFunc(tweets, isDiscordMainEmbedPossiblyCorrect) {
		s.Logger.Info("Tweets are possibly embeddable, waiting...")
		time.Sleep(3 * time.Second)

		var err error
		existingEmbeds, err = s.GetMessageEmbeds()
		if err != nil {
			s.Logger.With(zap.Error(err)).Warn("Failed to get message embeds")
			return
		}
	}

	for _, tweet := range tweets {
		if isDiscordMainEmbedPossiblyCorrect(tweet) {
			existingEmbed := findTweetEmbed(tweet, existingEmbeds)
			if existingEmbed != nil && isDiscordMainEmbedCorrect(tweet, discord.UpdatableMessageEmbeds{existingEmbed}) {
				continue
			}
		}

		s.SendEmbeds(tweet.GetEmbeds())
	}
}

func findTweetEmbed(tweet *twitter.Tweet, embeds discord.UpdatableMessageEmbeds) *discord.UpdatableMessageEmbed {
	for _, embed := range embeds {
		matches := twitter.URLRegex.FindStringSubmatch(embed.URL)
		if len(matches) > 1 && matches[1] == tweet.ID {
			return embed
		}
	}

	return nil
}

func isDiscordMainEmbedCorrect(tweet *twitter.Tweet, existingEmbeds discord.UpdatableMessageEmbeds) bool {
	if !isDiscordMainEmbedPossiblyCorrect(tweet) {
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

	// Additional checks for tweets with photos
	if tweet.HasPhotos() {
		// The embed is missing the photo(s)
		if existingEmbeds[0].Image == nil {
			return false
		}

		// The photo URL is not OK
		resp, err := http.Get(existingEmbeds[0].Image.URL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return false
		}

		// More than one photo case filtered out by isNotEmbeddableByDiscord

		// First media is video or GIF case filtered out by isNotEmbeddableByDiscord

		// At this point, we know:
		// (1) The existing embed has photos
		// (2) The tweet only has one photo (and potentially videos in some order)
		// (3) The first media of this tweet is not a video or a GIF, hence is a photo
		// (2) and (3) imply that the tweet's first media is the only photo
		// Combied with (1), this means that the tweet's only photo is correctly embedded
	}

	return true
}

func isDiscordMainEmbedPossiblyCorrect(tweet *twitter.Tweet) bool {
	// Discord doesn't embed what it thinks is adult content
	if tweet.IsSensitive {
		return false
	}

	// Discord can't embed polls
	if tweet.Poll != nil {
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
		// Discord can't embed more than one photo
		if len(tweet.Photos()) > 1 {
			return false
		}

		// If the first media is a video or a GIF (and the tweet has other photos)
		// Then Discord will wrongly embed it as a photo, and will not embed the rest of the photos
		if tweet.Medias[0].Type == twitter.MediaTypeVideo || tweet.Medias[0].Type == twitter.MediaTypeGIF {
			return false
		}
	}

	// Discord might have a chance here
	// Possible cases
	// 0 photo case
	// 1 photo case
	// 1 video or GIF case (don't need to send the main embed if Discord sends it, we just need to send the video or GIF)
	// First media is photo, with everything else being videos or GIFs (same as above case)

	return true
}
