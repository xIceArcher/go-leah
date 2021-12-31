package twitter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/go-resty/resty/v2"
	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	tweetModeExtended = "extended"

	v2APIBaseURL     = "https://api.twitter.com/2"
	v2APIGetSpaceURL = v2APIBaseURL + "/spaces"
)

const (
	CacheKeyTwitterAPITweetFormat = "go-leah/twitterAPI/tweet/%s"
)

var URLRegex *regexp.Regexp = regexp.MustCompile(`(?:http[s]?://)?twitter\.com/[^/]*/status/([0-9]*)(?:\?[^ \r\n]*)?`)
var ErrNotFound = errors.New("resource not found")

type API interface {
	GetTweet(id string) (*Tweet, error)

	GetUser(id string) (*User, error)
	GetUserByScreenName(screenName string) (*User, error)
	GetUserTimeline(id string, sinceID string) ([]*Tweet, error)
	GetLastTweetID(userID string) (tweetID string, err error)
	GetSpace(spaceID string) (*Space, error)
}

type CachedAPI struct {
	*BaseAPI

	cache  cache.Cache
	logger *zap.SugaredLogger
}

func NewCachedAPI(cfg *config.TwitterConfig, c cache.Cache, logger *zap.SugaredLogger) API {
	_ = NewBaseAPI(cfg)

	return &CachedAPI{
		BaseAPI: &BaseAPI{},

		cache:  c,
		logger: logger,
	}
}

func (a *CachedAPI) GetTweet(id string) (*Tweet, error) {
	cacheKey := fmt.Sprintf(CacheKeyTwitterAPITweetFormat, id)
	logger := a.logger.With(zap.String("cacheKey", cacheKey))

	if tweet, err := func() (tweet *Tweet, err error) {
		val, err := a.cache.Get(context.Background(), cacheKey)
		if err != nil {
			return nil, err
		}

		valStr, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("unknown cache return type %T", val)
		}

		tweet = &Tweet{}
		err = json.Unmarshal([]byte(valStr), tweet)
		return
	}(); err == nil {
		return tweet, nil
	}

	tweet, err := a.BaseAPI.GetTweet(id)
	if err != nil {
		return nil, err
	}

	tweetBytes, err := json.Marshal(tweet)
	if err != nil {
		// This error only affects caching, ignore and return the result
		logger.With(zap.Error(err)).Warn("Failed to marshal tweet")
		return tweet, nil
	}

	if err := a.cache.SetWithExpiry(context.Background(), cacheKey, tweetBytes, 4*time.Minute); err != nil {
		logger.With(zap.Error(err)).Warn("Failed to set cache")
	}

	return tweet, nil
}

type BaseAPI struct{}

var (
	api                 *twitter.Client
	apiV2Client         *resty.Client
	expandIgnoreRegexes []*regexp.Regexp

	apiSetupOnce sync.Once
)

func NewBaseAPI(cfg *config.TwitterConfig) *BaseAPI {
	apiSetupOnce.Do(func() {
		config := oauth1.NewConfig(cfg.ConsumerKey, cfg.ConsumerSecret)
		token := oauth1.NewToken(cfg.AccessToken, cfg.AccessSecret)
		httpClient := config.Client(oauth1.NoContext, token)
		api = twitter.NewClient(httpClient)

		v2Config := clientcredentials.Config{
			ClientID:     cfg.ConsumerKey,
			ClientSecret: cfg.ConsumerSecret,
			TokenURL:     "https://api.twitter.com/oauth2/token",
		}
		apiV2Client = resty.NewWithClient(v2Config.Client(context.Background()))

		for _, r := range cfg.ExpandIgnoreRegexes {
			regex, err := regexp.Compile(r)
			if err != nil {
				zap.S().Warn(fmt.Sprintf("Regex %s is invalid", regex))
				continue
			}

			expandIgnoreRegexes = append(expandIgnoreRegexes, regex)
		}

	})

	return &BaseAPI{}
}

func (a *BaseAPI) GetTweet(id string) (*Tweet, error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}

	tweet, resp, err := api.Statuses.Show(idInt, &twitter.StatusShowParams{
		TweetMode: tweetModeExtended,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	t, err := a.parseTweet(tweet)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (a *BaseAPI) parseTweet(tweet *twitter.Tweet) (*Tweet, error) {
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
			t.URLs = append(t.URLs, utils.NewEntityWithReplacement(stringIdx, url.URL, url.ExpandedURL))
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

func (a *BaseAPI) getPhotoURLs(tweet *twitter.Tweet) (photoURLs []string) {
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

func (a *BaseAPI) getVideoURL(tweet *twitter.Tweet) (videoURL string) {
	if tweet.ExtendedEntities == nil {
		return
	}

	medias := tweet.ExtendedEntities.Media
	if len(medias) == 0 {
		return ""
	}

	maxBitrate := -1
	maxUrl := ""

	for _, video := range medias[0].VideoInfo.Variants {
		if video.ContentType == "video/mp4" && video.Bitrate > maxBitrate {
			maxBitrate = video.Bitrate
			maxUrl = video.URL
		}
	}

	return maxUrl
}

func (a *BaseAPI) GetUser(id string) (*User, error) {
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

func (a *BaseAPI) GetUserByScreenName(screenName string) (*User, error) {
	user, resp, err := api.Users.Show(&twitter.UserShowParams{
		ScreenName: screenName,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return a.parseUser(user)
}

func (a *BaseAPI) parseUser(u *twitter.User) (*User, error) {
	return &User{
		ID:              u.IDStr,
		Name:            u.Name,
		ScreenName:      u.ScreenName,
		ProfileImageURL: u.ProfileImageURLHttps,
	}, nil
}

func (a *BaseAPI) GetUserTimeline(id string, sinceID string) (ret []*Tweet, err error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}

	sinceIDInt, err := strconv.ParseInt(sinceID, 10, 64)
	if err != nil {
		return nil, err
	}

	tweets, resp, err := api.Timelines.UserTimeline(&twitter.UserTimelineParams{
		UserID:          idInt,
		SinceID:         sinceIDInt,
		TrimUser:        twitter.Bool(false),
		ExcludeReplies:  twitter.Bool(false),
		IncludeRetweets: twitter.Bool(true),
		TweetMode:       tweetModeExtended,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	for _, tweet := range tweets {
		t, err := a.parseTweet(&tweet)
		if err != nil {
			return nil, err
		}

		ret = append(ret, t)
	}

	return ret, nil
}

func (a *BaseAPI) GetLastTweetID(userID string) (tweetID string, err error) {
	idInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return "", err
	}

	tweets, resp, err := api.Timelines.UserTimeline(&twitter.UserTimelineParams{
		UserID:          idInt,
		Count:           5,
		TrimUser:        twitter.Bool(false),
		ExcludeReplies:  twitter.Bool(false),
		IncludeRetweets: twitter.Bool(true),
		TweetMode:       tweetModeExtended,
	})
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}
	if len(tweets) == 0 {
		return "", ErrNotFound
	}

	return tweets[0].IDStr, nil
}

func (a *BaseAPI) GetSpace(spaceID string) (*Space, error) {
	resp := &getSpaceResponse{}
	_, err := apiV2Client.R().
		SetQueryParams(map[string]string{
			"space.fields": "started_at,ended_at,title,participant_count",
			"expansions":   "creator_id",
			"user.fields":  "profile_image_url",
		}).
		SetResult(resp).
		Get(v2APIGetSpaceURL + "/" + spaceID)
	if err != nil {
		return nil, err
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf(resp.Errors[0].Detail)
	}

	return resp.toDTO(), nil
}
