package twitter

import (
	"fmt"
	"time"

	"github.com/xIceArcher/go-leah/utils"
)

const (
	MediaTypePhoto = "photo"
	MediaTypeVideo = "video"
)

type Tweet struct {
	ID        string
	User      *User
	Text      string
	Timestamp time.Time

	Photos []*Photo

	VideoURLs []string

	IsRetweet       bool
	RetweetedStatus *Tweet

	IsQuoted     bool
	QuotedStatus *Tweet

	IsReply   bool
	ReplyUser *User

	Hashtags     []*utils.Entity
	UserMentions []*utils.Entity
	MediaLinks   []*utils.Entity
	URLs         []*utils.Entity
}

func (t *Tweet) URL() string {
	return fmt.Sprintf("https://twitter.com/%s/status/%s", t.User.ScreenName, t.ID)
}

func (t *Tweet) DisplayText() string {
	if t.RetweetedStatus != nil {
		return t.RetweetedStatus.Text
	}

	return t.Text
}

func (t *Tweet) GetBaseID() string {
	if t.IsRetweet {
		return t.RetweetedStatus.ID
	}

	return t.ID
}

func (t *Tweet) HasPhotos() bool {
	return len(t.Photos) > 0
}

func (t *Tweet) HasVideos() bool {
	return len(t.VideoURLs) > 0
}

type User struct {
	ID              string
	Name            string
	ScreenName      string
	ProfileImageURL string
}

func (u *User) URL() string {
	return fmt.Sprintf("https://twitter.com/%s", u.ScreenName)
}

type Photo struct {
	URL     string
	AltText string
}

type SpaceState string

const (
	SpaceStateLive  SpaceState = "live"
	SpaceStateEnded SpaceState = "ended"
)

type Space struct {
	ID    string
	Title string
	State SpaceState

	Creator          *User
	ParticipantCount int

	StartTime time.Time
	EndTime   time.Time
}

func (s *Space) URL() string {
	return fmt.Sprintf("https://twitter.com/i/spaces/%s", s.ID)
}
