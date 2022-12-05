package tiktok

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/xIceArcher/go-leah/utils"
)

type TiktokResp struct {
	RawItem map[string]RawItem `json:"ItemModule"`
	RawUser struct {
		Users map[string]RawUser `json:"users"`
	} `json:"UserModule"`
}

type RawItem struct {
	CreateTime     string          `json:"createTime"`
	AuthorUniqueID string          `json:"author"`
	Description    string          `json:"desc"`
	Music          RawMusic        `json:"music"`
	Video          RawVideo        `json:"video"`
	Stats          RawStats        `json:"stats"`
	TextExtra      []*RawTextExtra `json:"textExtra"`
}

type RawVideo struct {
	DownloadAddr string `json:"downloadAddr"`
}

type RawMusic struct {
	ID         string `json:"id"`
	Album      string `json:"album"`
	AuthorName string `json:"authorName"`
	Title      string `json:"title"`
}

type RawStats struct {
	CommentCount uint64 `json:"commentCount"`
	DiggCount    uint64 `json:"diggCount"`
	ShareCount   uint64 `json:"shareCount"`
}

type RawUser struct {
	ID           string `json:"id"`
	UniqueID     string `json:"uniqueId"`
	Nickname     string `json:"Nickname"`
	AvatarLarger string `json:"avatarLarger"`
	AvatarMedium string `json:"avatarMedium"`
	AvatarThumb  string `json:"avatarThumb"`
}

type RawTextExtra struct {
	Start        int    `json:"start"`
	End          int    `json:"end"`
	HashtagName  string `json:"hashtagName"`
	UserUniqueID string `json:"userUniqueId"`
}

type Video struct {
	ID          string
	Description string
	Video       io.ReadCloser

	Music  *Music
	Author *User

	LikeCount    uint64
	CommentCount uint64
	ShareCount   uint64

	Tags     []*utils.Entity
	Mentions []*utils.Entity

	CreateTime time.Time
}

func (v *Video) URL() string {
	return fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", v.Author.UniqueID, v.ID)
}

type Music struct {
	ID         string
	Album      string
	AuthorName string
	Title      string
}

func (m *Music) String() string {
	title := strings.TrimSuffix(m.Title, fmt.Sprintf(" - %s", m.AuthorName))
	return fmt.Sprintf("%s - %s", title, m.AuthorName)
}

func (m *Music) URL() string {
	return fmt.Sprintf("https://www.tiktok.com/music/%s-%s", url.PathEscape(m.Title), m.ID)
}

type User struct {
	ID        string
	UniqueID  string
	Nickname  string
	AvatarURL string
}

func (u *User) URL() string {
	return fmt.Sprintf("https://www.tiktok.com/@%s", u.UniqueID)
}
