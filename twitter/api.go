package twitter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/xIceArcher/go-leah/cache"
	"go.uber.org/zap"
	"golang.org/x/net/html"
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

	tweet := rawResp.Tweet.ToDTO()
	if !rawResp.Tweet.PossiblySensitive {
		tweet.IsSensitive = a.isAdultRatedTweet(rawResp.Tweet)
	}

	return tweet, nil
}

func (a *BaseAPI) isAdultRatedTweet(tweet rawTweet) bool {
	screenName := "i"
	if tweet.Author.ScreenName != "" {
		screenName = tweet.Author.ScreenName
	}

	req, err := retryablehttp.NewRequest(http.MethodGet, fmt.Sprintf("https://fxtwitter.com/%s/status/%s", screenName, tweet.ID), nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}

	return isAdultRatedHTML(resp.Body)
}

func isAdultRatedHTML(reader io.Reader) bool {
	document, err := html.Parse(reader)
	if err != nil {
		return false
	}

	var findMeta func(*html.Node) bool
	findMeta = func(node *html.Node) bool {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "meta") {
			var name string
			var content string
			for _, attribute := range node.Attr {
				switch {
				case strings.EqualFold(attribute.Key, "name"):
					name = attribute.Val
				case strings.EqualFold(attribute.Key, "content"):
					content = attribute.Val
				}
			}
			if strings.EqualFold(name, "rating") && strings.EqualFold(strings.TrimSpace(content), "adult") {
				return true
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if findMeta(child) {
				return true
			}
		}
		return false
	}

	return findMeta(document)
}
