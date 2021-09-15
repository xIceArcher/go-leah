package twitter

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
)

const (
	tweetModeExtended = "extended"
)

var URLRegex *regexp.Regexp = regexp.MustCompile(`(?:http[s]?://)?twitter\.com/[^/]*/status/([0-9]*)(?:\?[^ \r\n]*)?`)
var ErrNotFound = errors.New("resource not found")

type API struct{}

var (
	api          *twitter.Client
	apiSetupOnce sync.Once
)

func NewAPI(cfg *config.TwitterConfig) API {
	apiSetupOnce.Do(func() {
		config := oauth1.NewConfig(cfg.ConsumerKey, cfg.ConsumerSecret)
		token := oauth1.NewToken(cfg.AccessToken, cfg.AccessSecret)
		httpClient := config.Client(oauth1.NoContext, token)

		api = twitter.NewClient(httpClient)
	})

	return API{}
}

func (a *API) GetTweet(id string) (*Tweet, error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}

	tweet, resp, err := api.Statuses.Show(idInt, &twitter.StatusShowParams{
		TweetMode: tweetModeExtended,
	})
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	t, err := a.parseTweet(tweet)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (a *API) parseTweet(tweet *twitter.Tweet) (*Tweet, error) {
	timestamp, err := time.Parse(time.RubyDate, tweet.CreatedAt)
	if err != nil {
		return nil, err
	}

	text := tweet.FullText
	if text == "" {
		text = tweet.Text
	}

	u, err := a.parseUser(tweet.User)
	if err != nil {
		return nil, err
	}

	photoURLs := a.getPhotoURLs(tweet)
	videoURL := a.getVideoURL(tweet)

	t := &Tweet{
		ID:        tweet.IDStr,
		User:      u,
		Text:      text,
		Timestamp: timestamp,

		HasPhotos: len(photoURLs) != 0,
		PhotoURLs: photoURLs,

		HasVideo: videoURL != "",
		VideoURL: videoURL,
	}

	if tweet.Entities != nil {
		runes := []rune(t.Text)

		for _, hashtag := range tweet.Entities.Hashtags {
			hashSymbol := string(runes[hashtag.Indices.Start()])
			stringIdx := utils.GetStringIdx(runes, hashtag.Indices.Start())

			t.Hashtags = append(t.Hashtags, utils.NewEntity(stringIdx, hashSymbol+hashtag.Text))
		}

		for _, mention := range tweet.Entities.UserMentions {
			atSymbol := string(runes[mention.Indices.Start()])
			stringIdx := utils.GetStringIdx(runes, mention.Indices.Start())

			t.UserMentions = append(t.UserMentions, utils.NewEntity(stringIdx, atSymbol+mention.ScreenName))
		}

		for _, mediaLink := range tweet.Entities.Media {
			stringIdx := utils.GetStringIdx(runes, mediaLink.Indices.Start())
			t.MediaLinks = append(t.MediaLinks, utils.NewEntity(stringIdx, mediaLink.URL))
		}

		for _, url := range tweet.Entities.Urls {
			stringIdx := utils.GetStringIdx(runes, url.Indices.Start())
			t.URLs = append(t.URLs, utils.NewEntity(stringIdx, url.URL))
		}
	}

	if tweet.RetweetedStatus != nil {
		t.IsRetweet = true
		retweetedStatus, err := a.parseTweet(tweet.RetweetedStatus)
		if err == nil {
			t.RetweetedStatus = retweetedStatus
		}
	}

	if tweet.QuotedStatus != nil {
		t.IsQuoted = true
		quotedStatus, err := a.parseTweet(tweet.QuotedStatus)
		if err == nil {
			t.QuotedStatus = quotedStatus
		}
	}

	if tweet.InReplyToUserIDStr != "" {
		t.IsReply = true
		replyUser, err := a.GetUser(tweet.InReplyToUserIDStr)
		if err == nil {
			t.ReplyUser = replyUser
		}
	}

	return t, nil
}

func (a *API) getPhotoURLs(tweet *twitter.Tweet) (photoURLs []string) {
	if tweet.ExtendedEntities == nil {
		return
	}

	for _, media := range tweet.ExtendedEntities.Media {
		if media.Type == MediaTypePhoto {
			photoURLs = append(photoURLs, media.MediaURL)
		}
	}

	return photoURLs
}

func (a *API) getVideoURL(tweet *twitter.Tweet) (videoURL string) {
	if tweet.ExtendedEntities == nil {
		return
	}

	for _, media := range tweet.ExtendedEntities.Media {
		if media.Type == MediaTypeVideo {
			return media.MediaURL
		}
	}

	return ""
}

func (a *API) GetUser(id string) (*User, error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}

	user, resp, err := api.Users.Show(&twitter.UserShowParams{
		UserID: idInt,
	})
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a.parseUser(user)
}

func (a *API) parseUser(u *twitter.User) (*User, error) {
	return &User{
		ID:              u.IDStr,
		Name:            u.Name,
		ScreenName:      u.ScreenName,
		ProfileImageURL: u.ProfileImageURLHttps,
	}, nil
}
