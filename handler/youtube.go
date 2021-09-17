package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/utils"
	"github.com/xIceArcher/go-leah/youtube"
	"go.uber.org/zap"
)

func YoutubeLiveStream(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, videoIDs []string) (err error) {
	api, err := youtube.NewAPI(cfg.Google)
	if err != nil {
		return err
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(videoIDs))

	for _, videoID := range videoIDs {
		if len(embeds) == 10 {
			zap.S().Warn("More than 10 embeds in one message")
			break
		}

		logger := zap.S().With(
			"videoID", videoID,
		)

		videoInfo, err := api.GetVideoInfo(videoID, []string{youtube.PartLiveStreamingDetails, youtube.PartContentDetails, youtube.PartSnippet})
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video info")
			continue
		}

		if videoInfo.Snippet.LiveBroadcastContent == youtube.LiveBroadcastContentNone {
			logger.Info("Not a livestream")
			continue
		}

		videoThumbnail, err := api.GetBestThumbnail(videoInfo.Snippet.Thumbnails)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get video thumbnail")
			continue
		}

		channelInfo, err := api.GetChannelInfo(videoInfo.Snippet.ChannelId, []string{youtube.PartSnippet})
		if err != nil {
			logger.With(zap.Error(err)).Error("Get channel info")
			continue
		}

		channelThumbnail, err := api.GetBestThumbnail(channelInfo.Snippet.Thumbnails)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get channel thumbnail")
			continue
		}

		actualStart, actualStartOk := utils.ParseISOTime(videoInfo.LiveStreamingDetails.ActualStartTime)
		scheduledStart, scheduledStartOk := utils.ParseISOTime(videoInfo.LiveStreamingDetails.ScheduledStartTime)

		var startTime time.Time
		if !actualStartOk && !scheduledStartOk {
			logger.With(
				zap.String("actualStart", videoInfo.LiveStreamingDetails.ActualStartTime),
				zap.String("scheduledStart", videoInfo.LiveStreamingDetails.ScheduledStartTime),
			).Error("Cannot parse start time")
			continue
		} else if !actualStartOk {
			startTime = scheduledStart
		} else if !scheduledStartOk {
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

			if videoInfo.LiveStreamingDetails.ConcurrentViewers > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   "Viewers",
					Value:  fmt.Sprint(videoInfo.LiveStreamingDetails.ConcurrentViewers),
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

		duration, ok := utils.ParseISODuration(videoInfo.ContentDetails.Duration)
		if ok && duration > time.Duration(0) {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Duration",
				Value:  utils.FormatDurationSimple(duration),
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

		embeds = append(embeds, &discordgo.MessageEmbed{
			URL:   api.GetVideoURL(videoID),
			Title: videoInfo.Snippet.Title,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: videoThumbnail.Url,
			},
			Author: &discordgo.MessageEmbedAuthor{
				Name:    videoInfo.Snippet.ChannelTitle,
				URL:     api.GetChannelURL(videoInfo.Snippet.ChannelId),
				IconURL: channelThumbnail.Url,
			},
			Timestamp: startTime.Format(time.RFC3339),
			Fields:    fields,
			Color:     utils.ParseHexColor(color),
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "YouTube",
				IconURL: "https://cdn4.iconfinder.com/data/icons/social-media-2210/24/Youtube-512.png",
			},
		})
	}

	if len(embeds) > 0 {
		_, err = session.ChannelMessageSendEmbeds(msg.ChannelID, embeds)
	}
	return err
}
