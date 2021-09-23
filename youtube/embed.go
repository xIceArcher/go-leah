package youtube

import (
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/utils"
)

var (
	ErrNotLivestream = errors.New("not a livestreams")
	ErrNoStartTime   = errors.New("cannot parse start time")
)

func (v *Video) GetEmbed(onlyLivestream bool) (embed *discordgo.MessageEmbed, err error) {
	if onlyLivestream && v.LiveStreamingDetails == nil {
		return nil, ErrNotLivestream
	}

	actualStart := v.LiveStreamingDetails.ActualStartTime
	scheduledStart := v.LiveStreamingDetails.ScheduledStartTime

	if actualStart.IsZero() && scheduledStart.IsZero() {
		return nil, ErrNoStartTime
	}

	var startTime time.Time
	if actualStart.IsZero() {
		startTime = scheduledStart
	} else if scheduledStart.IsZero() {
		startTime = actualStart
	} else {
		if actualStart.Before(scheduledStart) {
			startTime = actualStart
		} else {
			startTime = scheduledStart
		}
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	if time.Now().After(startTime) {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Started",
			Value:  utils.FormatDiscordRelativeTime(startTime),
			Inline: true,
		})

		if v.LiveStreamingDetails.ConcurrentViewers > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Viewers",
				Value:  fmt.Sprint(v.LiveStreamingDetails.ConcurrentViewers),
				Inline: true,
			})
		}
	} else {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Starts",
			Value:  utils.FormatDiscordRelativeTime(startTime),
			Inline: true,
		})
	}

	if v.Duration > time.Duration(0) {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Duration",
			Value:  utils.FormatDurationSimple(v.Duration),
			Inline: true,
		})
	}

	var color string
	if time.Now().After(startTime) {
		color = consts.ColorRed
	} else if time.Until(startTime) < time.Hour {
		color = consts.ColorAmber
	} else {
		color = consts.ColorGreen
	}

	return &discordgo.MessageEmbed{
		URL:   v.URL(),
		Title: v.Title,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: v.ThumbnailURL,
		},
		Author: &discordgo.MessageEmbedAuthor{
			Name:    v.Channel.Title,
			URL:     v.Channel.URL(),
			IconURL: v.Channel.ThumbnailURL,
		},
		Timestamp: startTime.Format(time.RFC3339),
		Fields:    fields,
		Color:     utils.ParseHexColor(color),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "YouTube",
			IconURL: "https://cdn4.iconfinder.com/data/icons/social-media-2210/24/Youtube-512.png",
		},
	}, nil
}
