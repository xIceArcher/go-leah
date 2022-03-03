package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/config"
)

type API struct{}

var (
	instaPostURLFormat  string
	instaStoryURLFormat string
	client              *http.Client
	apiSetupOnce        sync.Once
)

func NewAPI(cfg *config.InstaConfig) (*API, error) {
	apiSetupOnce.Do(func() {
		instaPostURLFormat = cfg.PostURLFormat
		instaStoryURLFormat = cfg.StoryURLFormat
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	})

	return &API{}, nil
}

func (API) GetPost(shortcode string) (*Post, error) {
	resp, err := client.Get(fmt.Sprintf(instaPostURLFormat, shortcode))
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

	rawResp := RawPostResp{}
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

func (API) GetStory(username string, storyID string) (*Story, error) {
	resp, err := client.Get(fmt.Sprintf(instaStoryURLFormat, username))
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

	rawResp := RawReel{}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	if len(rawResp.ReelMedia) == 0 {
		return nil, errors.New("failed to get stories")
	}

	var rawReel *RawReelMedia
	if storyID != "" {
		for _, reel := range rawResp.ReelMedia {
			if strings.HasPrefix(reel.ID, storyID) {
				rawReel = reel
				break
			}
		}

		if rawReel == nil {
			return nil, errors.New("failed to get story")
		}
	} else {
		latestReelIdx := 0
		for i, reel := range rawResp.ReelMedia {
			if reel.TakenAtTimestamp > rawResp.ReelMedia[latestReelIdx].TakenAtTimestamp {
				latestReelIdx = i
			}
		}
		rawReel = rawResp.ReelMedia[latestReelIdx]
	}

	fullName := rawResp.User.FullName
	if fullName == "" {
		fullName = rawResp.User.Username
	}

	return &Story{
		Owner: &User{
			Username:      rawResp.User.Username,
			Fullname:      fullName,
			ProfilePicURL: rawResp.User.ProfilePicURL,
		},

		Timestamp: time.Unix(rawReel.TakenAtTimestamp, 0),
		MediaURL:  rawReel.extractMediaURL(),
		MediaType: rawReel.MediaType,
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
