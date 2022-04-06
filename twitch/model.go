package twitch

import (
	"fmt"
	"time"
)

type Stream struct {
	Title        string
	ThumbnailURL string

	User *User

	ViewerCount int
	StartedAt   time.Time
}

func (s *Stream) URL() string {
	return s.User.URL()
}

type User struct {
	LoginName       string
	Name            string
	ProfileImageURL string
}

func (u *User) URL() string {
	return fmt.Sprintf("https://twitch.tv/%s", u.LoginName)
}
