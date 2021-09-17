package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/twitch"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

func TwitchLiveStream(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, loginNames []string) (err error) {
	api, err := twitch.NewAPI(cfg.Twitch)
	if err != nil {
		return nil
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(loginNames))

	for _, loginName := range loginNames {
		logger := zap.S().With(
			"loginName", loginName,
		)

		streamInfo, err := api.GetStream(loginName)
		if err == twitch.ErrNotFound {
			logger.Info("Stream is not found or not live")
			return nil
		}
		if err != nil {
			return err
		}

		user, err := api.GetUser(loginName)
		var profileImageURL string
		if err == nil {
			profileImageURL = user.ProfileImageURL
		}

		fields := make([]*discordgo.MessageEmbedField, 0)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Started",
			Value:  utils.FormatDiscordRelativeTime(streamInfo.StartedAt),
			Inline: true,
		})
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Viewers",
			Value:  fmt.Sprint(streamInfo.ViewerCount),
			Inline: true,
		})

		embeds = append(embeds, &discordgo.MessageEmbed{
			URL:   api.GetUserURL(loginName),
			Title: streamInfo.Title,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: api.FormatThumbnailURL(streamInfo.ThumbnailURL, 1920, 1080),
			},
			Author: &discordgo.MessageEmbedAuthor{
				Name:    streamInfo.UserName,
				URL:     api.GetUserURL(loginName),
				IconURL: profileImageURL,
			},
			Timestamp: streamInfo.StartedAt.Format(time.RFC3339),
			Fields:    fields,
			Color:     utils.ParseHexColor(consts.ColorTwitch),
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "Twitch",
				IconURL: "https://cdn4.iconfinder.com/data/icons/logos-and-brands/512/343_Twitch_logo-512.png",
			},
		})
	}

	if len(embeds) > 0 {
		_, err = session.ChannelMessageSendEmbeds(msg.ChannelID, embeds)
	}
	return err
}
