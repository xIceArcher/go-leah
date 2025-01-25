package redbook

import (
	"strings"
	"time"
)

type RawRedbookRequest struct {
	URL string `json:"url"`
}

type RawRedbookResponse struct {
	Message string `json:"message"`
	URL     string `json:"url"`
	Data    struct {
		CollectionCount   string              `json:"收藏数量"`
		CommentCount      string              `json:"评论数量"`
		ShareCount        string              `json:"分享数量"`
		LikeCount         string              `json:"点赞数量"`
		Labels            string              `json:"作品标签"`
		ID                string              `json:"作品ID"`
		Title             string              `json:"作品标题"`
		Description       string              `json:"作品描述"`
		Type              RawRedbookMediaType `json:"作品类型"`
		CreateTimeStr     string              `json:"发布时间"`
		UpdateTime        string              `json:"最后更新时间"`
		AuthorNickname    string              `json:"作者昵称"`
		AuthorID          string              `json:"作者ID"`
		AuthorURL         string              `json:"作者链接"`
		MediaDownloadURLs []string            `json:"下载地址"`
	} `json:"data"`
}

func (r *RawRedbookResponse) CreateTime() time.Time {
	layout := "2006-01-02_15:04:05"

	loc, err := time.LoadLocation("Local")
	if err != nil {
		return time.Unix(0, 0)
	}

	ret, err := time.ParseInLocation(layout, r.Data.CreateTimeStr, loc)
	if err != nil {
		return time.Unix(0, 0)
	}
	return ret
}

func (r *RawRedbookResponse) CleanDescription() string {
	// TODO: Figure out a way to deal with emoji
	return strings.ReplaceAll(r.Data.Description, "[话题]#", "")
}

type RawRedbookMediaType string

const (
	RawRedbookMediaTypeVideo RawRedbookMediaType = "视频"
	RawRedbookMediaTypePhoto RawRedbookMediaType = "图文"
)

type Post struct {
	ID  string
	URL string

	Title       string
	Description string
	CreateTime  time.Time

	Author *Author

	PhotoURLs []string
	VideoURLs []string
}

type Author struct {
	ID  string
	URL string

	Name string
}
