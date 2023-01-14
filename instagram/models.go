package instagram

import (
	"fmt"
	"time"
)

type MediaType int

const (
	MediaTypeImage MediaType = 1
	MediaTypeVideo MediaType = 2
)

type RawPostResp struct {
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

type RawReel struct {
	ID   int64   `json:"id"`
	User RawUser `json:"user"`

	ReelMedia []*RawReelMedia `json:"items"`
}

type RawUserResp struct {
	User *RawUser `json:"user"`
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

type RawReelMedia struct {
	ID               string            `json:"id"`
	ImageVersions    *RawImageVersions `json:"image_versions2"`
	VideoVersions    []*RawVideo       `json:"video_versions"`
	MediaType        MediaType         `json:"media_type"`
	TakenAtTimestamp int64             `json:"taken_at"`
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
			if image.Width*image.Height > media.ImageVersions.Candidates[currBestIdx].Width*media.ImageVersions.Candidates[currBestIdx].Height {
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
			if video.Width*video.Height > media.VideoVersions[currBestIdx].Width*media.VideoVersions[currBestIdx].Height {
				currBestIdx = i
			}
		}

		videoURLs = append(videoURLs, media.VideoVersions[currBestIdx].URL)
	}

	return videoURLs
}

func (r *RawReelMedia) extractMediaURL() string {
	if r.MediaType == MediaTypeImage {
		return r.extractBestImageURL()
	} else if r.MediaType == MediaTypeVideo {
		return r.extractBestVideoURL()
	} else {
		return ""
	}
}

func (r *RawReelMedia) extractBestImageURL() string {
	currBestIdx := 0
	for i, image := range r.ImageVersions.Candidates {
		if image.Width*image.Height > r.ImageVersions.Candidates[currBestIdx].Width*r.ImageVersions.Candidates[currBestIdx].Height {
			currBestIdx = i
		}
	}

	return r.ImageVersions.Candidates[currBestIdx].URL
}

func (r *RawReelMedia) extractBestVideoURL() string {
	currBestIdx := 0
	for i, video := range r.VideoVersions {
		if video.Width*video.Height > r.VideoVersions[currBestIdx].Width*r.VideoVersions[currBestIdx].Height {
			currBestIdx = i
		}
	}

	return r.VideoVersions[currBestIdx].URL
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

type Story struct {
	ID        string
	Owner     *User
	Timestamp time.Time
	MediaURL  string
	MediaType MediaType
}

func (s *Story) URL() string {
	return fmt.Sprintf("https://www.instagram.com/stories/%s/%s", s.Owner.Username, s.ID)
}

func (s *Story) IsImage() bool {
	return s.MediaType == MediaTypeImage
}

func (s *Story) IsVideo() bool {
	return s.MediaType == MediaTypeVideo
}
