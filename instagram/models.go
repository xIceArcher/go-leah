package instagram

import (
	"fmt"
	"time"
)

type RawResp struct {
	EntryData struct {
		PostPage []struct {
			GraphQL struct {
				ShortcodeMedia RawPost `json:"shortcode_media"`
			} `json:"graphql"`
		} `json:"PostPage"`
	} `json:"entry_data"`
}

type RawEdge struct {
	Node struct {
		Text       string `json:"text"`
		DisplayURL string `json:"display_url"`
		IsVideo    bool   `json:"is_video"`
		VideoURL   string `json:"video_url"`
	} `json:"node"`
}

type RawEdges struct {
	Edges []RawEdge `json:"edges"`
	Count int       `json:"count"`
}

type RawUser struct {
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	ProfilePicURL string `json:"profile_pic_url"`
}

type RawPost struct {
	Shortcode string  `json:"shortcode"`
	Owner     RawUser `json:"owner"`

	EdgeSidecarToChildren RawEdges `json:"edge_sidecar_to_children"`
	EdgesMediaToCaption   RawEdges `json:"edge_media_to_caption"`
	EdgeMediaPreviewLike  RawEdges `json:"edge_media_preview_like"`

	DisplayURL string `json:"display_url"`
	IsVideo    bool   `json:"is_video"`
	VideoURL   string `json:"video_url"`

	TakenAtTimestamp int64 `json:"taken_at_timestamp"`
}

func (p *RawPost) extractPhotoURLs() []string {
	photoURLs := make([]string, 0)
	for _, edge := range p.EdgeSidecarToChildren.Edges {
		if edge.Node.IsVideo {
			continue
		}

		photoURLs = append(photoURLs, edge.Node.DisplayURL)
	}

	if len(photoURLs) == 0 {
		return []string{p.DisplayURL}
	}

	return photoURLs
}

func (p *RawPost) extractVideoURLs() []string {
	videoURLs := make([]string, 0)
	for _, edge := range p.EdgeSidecarToChildren.Edges {
		if !edge.Node.IsVideo {
			continue
		}

		videoURLs = append(videoURLs, edge.Node.VideoURL)
	}

	if len(videoURLs) == 0 && p.IsVideo {
		return []string{p.VideoURL}
	}

	return videoURLs
}

type User struct {
	Username      string
	Fullname      string
	ProfilePicURL string
}

func (u *User) URL() string {
	return fmt.Sprintf("https://instagram.com/%s", u.Username)
}

type Post struct {
	Shortcode string
	Owner     *User

	Text      string
	Likes     int
	Timestamp time.Time

	PhotoURLs []string
	VideoURLs []string
}

func (p *Post) URL() string {
	return fmt.Sprintf("https://instagram.com/p/%s", p.Shortcode)
}
