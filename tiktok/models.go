package tiktok

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xIceArcher/go-leah/utils"
)

type RawVideo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Timestamp   int64  `json:"timestamp"`

	LikeCount    uint64 `json:"like_count"`
	RepostCount  uint64 `json:"repost_count"`
	CommentCount uint64 `json:"comment_count"`

	Uploader   string `json:"uploader"`
	UploaderID string `json:"uploader_id"`
	Creator    string `json:"creator"`

	Track  string `json:"track"`
	Artist string `json:"artist"`

	Formats []*RawFormat `json:"formats"`
}

type RawFormat struct {
	FormatID string `json:"format_id"`
	Format   string `json:"format"`
	URL      string `json:"url"`

	HTTPHeaders map[string]string `json:"http_headers"`
	Cookies     string            `json:"cookies"`

	FormatNote string `json:"format_note"`
	VideoCodec string `json:"vcodec"`
}

type RawFormatType int

const (
	RawFormatTypeDownload RawFormatType = 1
	RawFormatTypeDirect   RawFormatType = 2
	RawFormatTypePlayback RawFormatType = 3
	RawFormatTypeUnknown  RawFormatType = 4
)

func (f *RawFormat) IsWatermarked() bool {
	return strings.Contains(strings.ToLower(f.FormatNote), "watermark")
}

type RawUser struct {
	DefaultScope struct {
		WebappUserDetail struct {
			UserInfo struct {
				User struct {
					ID           string `json:"id"`
					ShortID      string `json:"shortId"`
					UniqueID     string `json:"uniqueId"`
					Nickname     string `json:"nickname"`
					AvatarLarger string `json:"avatarLarger"`
					AvatarMedium string `json:"avatarMedium"`
					AvatarThumb  string `json:"avatarThumb"`
				} `json:"user"`
			} `json:"userInfo"`
		} `json:"webapp.user-detail"`
	} `json:"__DEFAULT_SCOPE__"`
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

type User struct {
	ID        string
	UniqueID  string
	Nickname  string
	AvatarURL string
}

func (u *User) URL() string {
	return fmt.Sprintf("https://www.tiktok.com/@%s", u.UniqueID)
}
