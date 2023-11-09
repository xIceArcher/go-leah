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
	URL         string            `json:"url"`
	Cookies     string            `json:"cookies"`
	HTTPHeaders map[string]string `json:"http_headers"`
	FormatNote  string            `json:"format_note"`
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

func (f *RawFormat) IsFromAPI() bool {
	return strings.Contains(strings.ToLower(f.FormatNote), "api")
}

func (f *RawFormat) DeduceType() RawFormatType {
	formatNote := strings.ToLower(f.FormatNote)
	if strings.Contains(formatNote, "download") {
		return RawFormatTypeDownload
	} else if strings.Contains(formatNote, "direct") {
		return RawFormatTypeDirect
	} else if strings.Contains(formatNote, "playback") {
		return RawFormatTypePlayback
	}

	return RawFormatTypeUnknown
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
