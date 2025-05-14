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
	ID               string     `json:"id"`
	URL              string     `json:"url"`
	Text             string     `json:"text"`
	CreatedAt        string     `json:"created_at"`
	CreatedTimestamp int        `json:"created_timestamp"`
	Author           rawAuthor  `json:"author"`
	Replies          int        `json:"replies"`
	Retweets         int        `json:"retweets"`
	Likes            int        `json:"likes"`
	Views            int        `json:"views"`
	Color            string     `json:"color"`
	TwitterCard      string     `json:"twitter_card"`
	Lang             string     `json:"lang"`
	Source           string     `json:"source"`
	ReplyingTo       *string    `json:"replying_to"`
	ReplyingToStatus *string    `json:"replying_to_status"`
	Quote            *rawTweet  `json:"quote"`
	Media            *rawMedias `json:"media"`
	Poll             *rawPoll   `json:"poll"`
}

type rawAuthor struct {
	Name        string `json:"name"`
	ScreenName  string `json:"screen_name"`
	AvatarURL   string `json:"avatar_url"`
	AvatarColor string `json:"avatar_color"`
	BannerURL   string `json:"banner_url"`
}

type rawMedias struct {
	Medias []*rawMedia `json:"all"`
}

type rawMedia struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	AltText string `json:"altText"`
}

type rawPoll struct {
	EndsAt  time.Time        `json:"ends_at"`
	Choices []*rawPollChoice `json:"choices"`
}

type rawPollChoice struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

func (t *rawTweet) ToDTO() *Tweet {
	if t == nil {
		return nil
	}

	medias := make([]*Media, 0)

	if t.Media != nil {
		for _, media := range t.Media.Medias {
			medias = append(medias, &Media{
				Type:    MediaType(media.Type),
				URL:     media.URL,
				AltText: media.AltText,
			})
		}
	}

	var poll *Poll
	if t.Poll != nil {
		poll = &Poll{
			EndsAt: t.Poll.EndsAt,
		}

		for _, choice := range t.Poll.Choices {
			poll.Choices = append(poll.Choices, &PollChoice{
				Label: choice.Label,
				Count: choice.Count,
			})
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

		Medias: medias,

		IsRetweet:       false,
		RetweetedStatus: nil,

		IsQuoted:     t.Quote != nil,
		QuotedStatus: t.Quote.ToDTO(),

		Poll: poll,
	}
}

type listTweetsFromListResponse struct {
	Tweets []struct {
		ID string `json:"id"`
	} `json:"tweets"`
	NextCursor string `json:"next_cursor"`
}
