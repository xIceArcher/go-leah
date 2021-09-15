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

	HasPhotos bool
	PhotoURLs []string

	HasVideo bool
	VideoURL string

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

type User struct {
	ID              string
	Name            string
	ScreenName      string
	ProfileImageURL string
}

func (u *User) URL() string {
	return fmt.Sprintf("https://twitter.com/%s", u.ScreenName)
}
