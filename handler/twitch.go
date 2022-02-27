package handler

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/twitch"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

type TwitchLiveStreamHandler struct {
	api *twitch.API

	RegexManager
	unimplementedHandler
}

func (TwitchLiveStreamHandler) String() string {
	return "twitchLiveStream"
}

func (h *TwitchLiveStreamHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp, wg *sync.WaitGroup) (err error) {
	h.Regexes = regexes
	h.api, err = twitch.NewAPI(cfg.Twitch)
	return err
}

func (h *TwitchLiveStreamHandler) Handle(ctx context.Context, session *discordgo.Session, channelID string, msg string, logger *zap.SugaredLogger) (loginNames []string, err error) {
	loginNames = h.Match(msg)
	embeds := make([]*discordgo.MessageEmbed, 0, len(loginNames))

	for _, loginName := range loginNames {
		logger := logger.With(
			zap.String("loginName", loginName),
		)

		streamInfo, err := h.api.GetStream(loginName)
		if errors.Is(err, twitch.ErrNotFound) {
			logger.Info("Stream is not found or not live")
			return loginNames, nil
		}
		if err != nil {
			return loginNames, err
		}

		user, err := h.api.GetUser(loginName)
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
			URL:   h.api.GetUserURL(loginName),
			Title: streamInfo.Title,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: h.api.FormatThumbnailURL(streamInfo.ThumbnailURL, 1920, 1080),
			},
			Author: &discordgo.MessageEmbedAuthor{
				Name:    streamInfo.UserName,
				URL:     h.api.GetUserURL(loginName),
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
		_, err = session.ChannelMessageSendEmbeds(channelID, embeds)
	}
	return loginNames, err
}
