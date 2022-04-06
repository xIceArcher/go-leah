package youtube

import (
	"fmt"
	"time"
)

type Video struct {
	ID        string
	ChannelID string

	Title    string
	Duration time.Duration

	ThumbnailURL string

	IsDone               bool
	LiveStreamingDetails *LiveStreamingDetails

	Channel *Channel
}

type LiveStreamingDetails struct {
	ActualStartTime    time.Time
	ScheduledStartTime time.Time
	ActualEndTime      time.Time

	ConcurrentViewers uint64
}

func (v *Video) URL() string {
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", v.ID)
}

func (v *Video) IsActiveLivestream() bool {
	return v.LiveStreamingDetails != nil && !v.IsDone
}

type Channel struct {
	ID    string
	Title string

	ThumbnailURL string
}

func (c *Channel) URL() string {
	return fmt.Sprintf("https://www.youtube.com/channel/%s", c.ID)
}
