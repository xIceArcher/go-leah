package instagram

import (
	"fmt"
	"time"
)

type RawResp struct {
	Items []RawPost `json:"items"`
}

type RawPost struct {
	Shortcode string `json:"code"`

	User RawUser `json:"user"`

	Caption struct {
		Text string `json:"text"`
	} `json:"caption"`

	CarouselMedia []*RawCarouselMedia `json:"carousel_media"`

	ImageVersions *RawImageVersions `json:"image_versions2"`
	VideoVersions []*RawVideo       `json:"video_versions"`

	LikeCount        int   `json:"like_count"`
	TakenAtTimestamp int64 `json:"taken_at"`
}

type RawUser struct {
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	ProfilePicURL string `json:"profile_pic_url"`
}

type RawCarouselMedia struct {
	ImageVersions *RawImageVersions `json:"image_versions2"`
	VideoVersions []*RawVideo       `json:"video_versions"`
}

func (m *RawCarouselMedia) IsVideo() bool {
	return len(m.VideoVersions) > 0
}

type RawImageVersions struct {
	Candidates []RawImage `json:"candidates"`
}

type RawImage struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
}

type RawVideo struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
}

func (p *RawPost) extractPhotoURLs() []string {
	photoURLs := make([]string, 0)

	if p.ImageVersions != nil && len(p.VideoVersions) == 0 {
		p.CarouselMedia = append(p.CarouselMedia, &RawCarouselMedia{
			ImageVersions: p.ImageVersions,
		})
	}

	for _, media := range p.CarouselMedia {
		if media.IsVideo() {
			continue
		}

		currBestIdx := 0
		for i, image := range media.ImageVersions.Candidates {
			if image.Width*image.Height > media.ImageVersions.Candidates[i].Width*media.ImageVersions.Candidates[i].Height {
				currBestIdx = i
			}
		}

		photoURLs = append(photoURLs, media.ImageVersions.Candidates[currBestIdx].URL)
	}

	return photoURLs
}

func (p *RawPost) extractVideoURLs() []string {
	videoURLs := make([]string, 0)

	if len(p.VideoVersions) > 0 {
		p.CarouselMedia = append(p.CarouselMedia, &RawCarouselMedia{
			VideoVersions: p.VideoVersions,
		})
	}

	for _, media := range p.CarouselMedia {
		if !media.IsVideo() {
			continue
		}

		currBestIdx := 0
		for i, video := range media.VideoVersions {
			if video.Width*video.Height > media.VideoVersions[i].Width*media.VideoVersions[i].Height {
				currBestIdx = i
			}
		}

		videoURLs = append(videoURLs, media.VideoVersions[currBestIdx].URL)
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
