package twitter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/xIceArcher/go-leah/cache"
	"go.uber.org/zap"
)

const (
	CacheKeyTwitterAPITweetFormat = "go-leah/twitterAPI/tweet/%s"
)

var URLRegex *regexp.Regexp = regexp.MustCompile(`(?:http[s]?://)?(?:(?:twitter)|(?:x))\.com/[^/]*/status/([0-9]*)(?:\?[^ \r\n]*)?`)
var ErrNotFound = errors.New("not found")
var ErrInternalServerError = errors.New("internal server error")

type API interface {
	GetTweet(id string) (*Tweet, error)
}

type CachedAPI struct {
	*BaseAPI

	cache  cache.Cache
	logger *zap.SugaredLogger
}

func NewCachedAPI(c cache.Cache, logger *zap.SugaredLogger) API {
	_ = NewBaseAPI()

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
	client *retryablehttp.Client

	apiSetupOnce sync.Once
)

func NewBaseAPI() *BaseAPI {
	apiSetupOnce.Do(func() {
		client = retryablehttp.NewClient()
		client.HTTPClient.Timeout = 30 * time.Second
		client.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
			// For some reason, the fxtwitter API will return 404 randomly
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return true, nil
			}

			return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
		}
		client.Logger = nil
	})

	return &BaseAPI{}
}

func (a *BaseAPI) GetTweet(id string) (*Tweet, error) {
	resp, err := client.Get(fmt.Sprintf("https://api.fxtwitter.com/a/status/%s", id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawResp := &getTweetResponse{}
	if err := json.Unmarshal(bytes, rawResp); err != nil {
		return nil, err
	}
	if rawResp.Code == http.StatusNotFound || rawResp.Code == http.StatusUnauthorized {
		return nil, ErrNotFound
	}
	if rawResp.Code == http.StatusInternalServerError {
		return nil, ErrInternalServerError
	}

	return rawResp.Tweet.ToDTO(), nil
}
