package twitter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
)

type ListResp struct {
	Tweets []struct {
		ID string `json:"id"`
	} `json:"tweets"`
}

func StalkList(ctx context.Context, c *config.Config, s *discord.Session, channelID string, listID string, interval time.Duration) error {
	since := time.Unix(0, 0)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	client := resty.New()

	twitterAPI := NewBaseAPI()

	startCh := make(chan struct{}, 1)
	startCh <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-startCh:
			url := "https://api.twitterapi.io/twitter/list/tweets"

			resp := &ListResp{}
			_, err := client.R().
				SetHeader("X-API-Key", c.Twitter.APIKey).
				SetQueryParams(map[string]string{
					"listId":    listID,
					"sinceTime": fmt.Sprintf("%v", since.Unix()),
				}).
				SetResult(resp).
				Get(url)

			if err != nil {
				zap.S().With(zap.Error(err)).Error("Got error when calling API")
				continue
			}

			if len(resp.Tweets) > 1 {
				resp.Tweets = resp.Tweets[0:1]
			}

			for _, tweet := range resp.Tweets {
				t, err := twitterAPI.GetTweet(tweet.ID)
				if err != nil {
					zap.S().With(zap.Error(err)).With(zap.String("id", tweet.ID)).Error("Got error when fetching tweet")
					continue
				}

				embeds := t.GetEmbeds()
				for _, embed := range embeds {
					if _, err := s.SendEmbed(channelID, embed); err != nil {
						zap.S().With(zap.Error(err)).With(zap.String("id", tweet.ID)).Error("Failed to send embed")
						continue
					}
				}

				zap.S().With(zap.String("id", tweet.ID)).Info("Posted")
			}

			since = time.Now()
		case <-ticker.C:
			url := "https://api.twitterapi.io/twitter/list/tweets"

			resp := &ListResp{}
			_, err := client.R().
				SetHeader("X-API-Key", c.Twitter.APIKey).
				SetQueryParams(map[string]string{
					"listId":    listID,
					"sinceTime": fmt.Sprintf("%v", since.Unix()),
				}).
				SetResult(resp).
				Get(url)

			if err != nil {
				zap.S().With(zap.Error(err)).Error("Got error when calling API")
				continue
			}

			if len(resp.Tweets) == 0 {
				zap.S().Info("Posted nothing")
			}

			for _, tweet := range resp.Tweets {
				t, err := twitterAPI.GetTweet(tweet.ID)
				if err != nil {
					zap.S().With(zap.Error(err)).With(zap.String("id", tweet.ID)).Error("Got error when fetching tweet")
					continue
				}

				embeds := t.GetEmbeds()
				for _, embed := range embeds {
					if _, err := s.SendEmbed(channelID, embed); err != nil {
						zap.S().With(zap.Error(err)).With(zap.String("id", tweet.ID)).Error("Failed to send embed")
						continue
					}

					zap.S().With(zap.String("id", tweet.ID)).Info("Posted")
				}
			}

			since = time.Now()
		}
	}
}
