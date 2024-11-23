package twitter

import (
	"fmt"
	"time"

	"github.com/xIceArcher/go-leah/utils"
)

type MediaType string

const (
	MediaTypePhoto MediaType = "photo"
	MediaTypeVideo MediaType = "video"
)

type Tweet struct {
	ID        string
	User      *User
	Text      string
	Timestamp time.Time

	Medias []*Media

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

	Poll *Poll
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
	return len(t.Photos()) > 0
}

func (t *Tweet) Photos() []*Media {
	ret := make([]*Media, 0)

	for _, m := range t.Medias {
		if m.Type == MediaTypePhoto {
			ret = append(ret, m)
		}
	}

	return ret
}

func (t *Tweet) HasVideos() bool {
	return len(t.Videos()) > 0
}

func (t *Tweet) Videos() []*Media {
	ret := make([]*Media, 0)

	for _, m := range t.Medias {
		if m.Type == MediaTypeVideo {
			ret = append(ret, m)
		}
	}

	return ret
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

type Media struct {
	Type    MediaType
	URL     string
	AltText string
}

type SpaceState string

const (
	SpaceStateLive  SpaceState = "live"
	SpaceStateEnded SpaceState = "ended"
)

type Poll struct {
	EndsAt  time.Time
	Choices []*PollChoice
}

func (p *Poll) IsEnded() bool {
	return time.Now().After(p.EndsAt)
}

type PollChoice struct {
	Label string
	Count int
}

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
