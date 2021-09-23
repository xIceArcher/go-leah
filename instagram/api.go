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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawResp := RawResp{}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	postPage := rawResp.EntryData.PostPage
	if len(postPage) == 0 {
		return nil, errors.New("failed to get post")
	}

	rawPost := postPage[0].GraphQL.ShortcodeMedia

	var text string
	if len(rawPost.EdgesMediaToCaption.Edges) != 0 {
		text = rawPost.EdgesMediaToCaption.Edges[0].Node.Text
	}

	return &Post{
		Shortcode: shortcode,
		Owner: &User{
			Username:      rawPost.Owner.Username,
			Fullname:      rawPost.Owner.FullName,
			ProfilePicURL: rawPost.Owner.ProfilePicURL,
		},

		Text:      text,
		Likes:     rawPost.EdgeMediaPreviewLike.Count,
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
