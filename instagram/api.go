package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	instaUserURLFormat  string
	client              *http.Client
	apiSetupOnce        sync.Once
)

func NewAPI(cfg *config.InstaConfig) (*API, error) {
	apiSetupOnce.Do(func() {
		instaPostURLFormat = cfg.PostURLFormat
		instaStoryURLFormat = cfg.StoryURLFormat
		instaUserURLFormat = cfg.UserURLFormat
		client = &http.Client{
			Timeout: 30 * time.Second,
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawResp := &RawPostResp{}
	if err := json.Unmarshal(body, rawResp); err != nil {
		return nil, err
	}

	if len(rawResp.Items) == 0 {
		return nil, errors.New("failed to get post")
	}

	return parsePost(&rawResp.Items[0]), nil
}

func (a *API) GetStory(username string, storyID string) (*Story, error) {
	resp, err := client.Get(fmt.Sprintf(instaStoryURLFormat, fmt.Sprintf("%s/%s", username, storyID)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawReelMedia := &RawReelMedia{}
	if err := json.Unmarshal(body, rawReelMedia); err != nil {
		return nil, err
	}

	user, err := getRawUser(username)
	if err != nil {
		return nil, err
	}

	return parseStory(rawReelMedia, user), nil
}

func (API) GetLatestStory(username string) (*Story, error) {
	resp, err := client.Get(fmt.Sprintf(instaStoryURLFormat, username))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawReel := &RawReel{}
	if err := json.Unmarshal(body, rawReel); err != nil {
		return nil, err
	}

	if len(rawReel.ReelMedia) == 0 {
		return nil, errors.New("failed to get stories")
	}

	latestReelIdx := 0
	for i, reel := range rawReel.ReelMedia {
		if reel.TakenAtTimestamp > rawReel.ReelMedia[latestReelIdx].TakenAtTimestamp {
			latestReelIdx = i
		}
	}

	return parseStory(rawReel.ReelMedia[latestReelIdx], &rawReel.User), nil
}

func (API) GetUser(username string) (*User, error) {
	rawUser, err := getRawUser(username)
	if err != nil {
		return nil, err
	}

	return parseUser(rawUser), nil
}

func getRawUser(username string) (*RawUser, error) {
	resp, err := client.Get(fmt.Sprintf(instaUserURLFormat, username))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawUser := &RawUserResp{}
	if err := json.Unmarshal(body, &rawUser); err != nil {
		return nil, err
	}

	return rawUser.User, nil
}

func parsePost(rawPost *RawPost) *Post {
	return &Post{
		Shortcode: rawPost.Shortcode,
		Owner:     parseUser(&rawPost.User),

		Text:      rawPost.Caption.Text,
		Likes:     rawPost.LikeCount,
		Timestamp: time.Unix(rawPost.TakenAtTimestamp, 0),

		PhotoURLs: rawPost.extractPhotoURLs(),
		VideoURLs: rawPost.extractVideoURLs(),
	}
}

func parseStory(rawReelMedia *RawReelMedia, rawUser *RawUser) *Story {
	return &Story{
		ID:    strings.Split(rawReelMedia.ID, "_")[0],
		Owner: parseUser(rawUser),

		Timestamp: time.Unix(rawReelMedia.TakenAtTimestamp, 0),
		MediaURL:  rawReelMedia.extractMediaURL(),
		MediaType: rawReelMedia.MediaType,
	}
}

func parseUser(rawUser *RawUser) *User {
	fullName := rawUser.FullName
	if fullName == "" {
		fullName = rawUser.Username
	}

	return &User{
		Username:      rawUser.Username,
		Fullname:      fullName,
		ProfilePicURL: rawUser.ProfilePicURL,
	}
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
