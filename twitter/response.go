package twitter

import (
	"time"
)

type getTweetResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Tweet   rawTweet `json:"tweet"`
}

type rawTweet struct {
	ID               string    `json:"id"`
	URL              string    `json:"url"`
	Text             string    `json:"text"`
	CreatedAt        string    `json:"created_at"`
	CreatedTimestamp int       `json:"created_timestamp"`
	Author           rawAuthor `json:"author"`
	Replies          int       `json:"replies"`
	Retweets         int       `json:"retweets"`
	Likes            int       `json:"likes"`
	Views            int       `json:"views"`
	Color            string    `json:"color"`
	TwitterCard      string    `json:"twitter_card"`
	Lang             string    `json:"lang"`
	Source           string    `json:"source"`
	ReplyingTo       *string   `json:"replying_to"`
	ReplyingToStatus *string   `json:"replying_to_status"`
	Quote            *rawTweet `json:"quote"`
	Media            *rawMedia `json:"media"`
}

type rawAuthor struct {
	Name        string `json:"name"`
	ScreenName  string `json:"screen_name"`
	AvatarURL   string `json:"avatar_url"`
	AvatarColor string `json:"avatar_color"`
	BannerURL   string `json:"banner_url"`
}

type rawMedia struct {
	Photos []*rawPhoto `json:"photos"`
	Videos []*rawVideo `json:"videos"`
}

type rawPhoto struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type rawVideo struct {
	URL          string  `json:"url"`
	ThumbnailURL string  `json:"thumbnail_url"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Duration     float64 `json:"duration"`
	Format       string  `json:"format"`
	Type         string  `json:"type"`
}

func (t *rawTweet) ToDTO() *Tweet {
	if t == nil {
		return nil
	}

	photoURLs := make([]string, 0)
	if t.Media != nil {
		for _, photo := range t.Media.Photos {
			photoURLs = append(photoURLs, photo.URL)
		}
	}

	videoURLs := make([]string, 0)
	if t.Media != nil && len(t.Media.Videos) > 0 {
		for _, video := range t.Media.Videos {
			videoURLs = append(videoURLs, video.URL)
		}
	}

	return &Tweet{
		ID: t.ID,
		User: &User{
			ID:              "",
			Name:            t.Author.Name,
			ScreenName:      t.Author.ScreenName,
			ProfileImageURL: t.Author.AvatarURL,
		},
		Text:      t.Text,
		Timestamp: time.Unix(int64(t.CreatedTimestamp), 0),

		PhotoURLs: photoURLs,
		VideoURLs: videoURLs,

		IsRetweet:       false,
		RetweetedStatus: nil,

		IsQuoted:     t.Quote != nil,
		QuotedStatus: t.Quote.ToDTO(),
	}
}
