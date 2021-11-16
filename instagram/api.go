package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/config"
)

type API struct{}

var (
	instaURLFormat string
	client         *http.Client
	apiSetupOnce   sync.Once
)

func NewAPI(cfg *config.InstaConfig) (*API, error) {
	apiSetupOnce.Do(func() {
		instaURLFormat = cfg.PostURLFormat
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	})

	return &API{}, nil
}

func (API) GetPost(shortcode string) (*Post, error) {
	resp, err := client.Get(fmt.Sprintf(instaURLFormat, shortcode))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawResp := RawResp{}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	if len(rawResp.Items) == 0 {
		return nil, errors.New("failed to get post")
	}

	rawPost := rawResp.Items[0]

	fullName := rawPost.User.FullName
	if fullName == "" {
		fullName = rawPost.User.Username
	}

	return &Post{
		Shortcode: shortcode,
		Owner: &User{
			Username:      rawPost.User.Username,
			Fullname:      fullName,
			ProfilePicURL: rawPost.User.ProfilePicURL,
		},

		Text:      rawPost.Caption.Text,
		Likes:     rawPost.LikeCount,
		Timestamp: time.Unix(rawPost.TakenAtTimestamp, 0),

		PhotoURLs: rawPost.extractPhotoURLs(),
		VideoURLs: rawPost.extractVideoURLs(),
	}, nil
}

func (API) GetHashtagURL(s string) string {
	firstChar := string([]rune(s)[0:1])
	if firstChar == "#" || firstChar == "＃" {
		s = string([]rune(s)[1:])
	}

	return fmt.Sprintf("https://www.instagram.com/explore/tags/%s", s)
}

func (API) GetMentionURL(s string) string {
	firstChar := string([]rune(s)[0:1])
	if firstChar == "@" || firstChar == "＠" {
		s = string([]rune(s)[1:])
	}

	return fmt.Sprintf("https://www.instagram.com/%s", s)
}
