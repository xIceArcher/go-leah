package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/instagram"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

func InstagramPost(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, shortcodes []string) (err error) {
	api, err := instagram.NewAPI(cfg.Instagram)
	if err != nil {
		return err
	}

	for _, shortcode := range shortcodes {
		logger := zap.S().With(
			zap.String("shortcode", shortcode),
		)

		post, err := api.GetPost(shortcode)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get post")
			continue
		}

		textWithEntities := &utils.TextWithEntities{Text: post.Text}
		textWithEntities.AddByRegex(instagram.MentionRegex, func(s string) string {
			return utils.GetDiscordNamedLink(s, api.GetMentionURL(s))
		})
		textWithEntities.AddByRegex(instagram.HashtagRegex, func(s string) string {
			return utils.GetDiscordNamedLink(s, api.GetHashtagURL(s))
		})

		segmentedText := textWithEntities.GetReplacedText(4096, -1)

		embeds := make([]*discordgo.MessageEmbed, 0)
		embeds = append(embeds, &discordgo.MessageEmbed{
			URL:   post.URL(),
			Title: fmt.Sprintf("Instagram post by %s", post.Owner.Fullname),
			Author: &discordgo.MessageEmbedAuthor{
				Name:    fmt.Sprintf("%s (%s)", post.Owner.Fullname, post.Owner.Username),
				URL:     post.Owner.URL(),
				IconURL: post.Owner.ProfilePicURL,
			},
			Description: segmentedText[0],
			Color:       utils.ParseHexColor(consts.ColorInsta),
		})

		for _, text := range segmentedText[1:] {
			embeds = append(embeds, &discordgo.MessageEmbed{
				Description: text,
				Color:       utils.ParseHexColor(consts.ColorInsta),
			})
		}

		embeds[len(embeds)-1].Image = &discordgo.MessageEmbedImage{
			URL: post.PhotoURLs[0],
		}
		embeds[len(embeds)-1].Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Likes",
				Value:  fmt.Sprintf("%v", post.Likes),
				Inline: true,
			},
		}

		for _, photoURL := range post.PhotoURLs[1:] {
			embeds = append(embeds, &discordgo.MessageEmbed{
				Image: &discordgo.MessageEmbedImage{
					URL: photoURL,
				},
				Color: utils.ParseHexColor(consts.ColorInsta),
			})
		}

		embeds[len(embeds)-1].Footer = &discordgo.MessageEmbedFooter{
			Text:    "Instagram",
			IconURL: "https://instagram-brand.com/wp-content/uploads/2016/11/Instagram_AppIcon_Aug2017.png?w=300",
		}
		embeds[len(embeds)-1].Timestamp = post.Timestamp.Format(time.RFC3339)

		if len(embeds) > 10 {
			zap.S().Warn("More than 10 embeds in one message")
		}

		if _, err = session.ChannelMessageSendEmbeds(msg.ChannelID, embeds); err != nil {
			logger.With(zap.Error(err)).Error("Send post embeds")
			continue
		}

		for _, videoURL := range post.VideoURLs {
			if _, err = session.ChannelMessageSend(msg.ChannelID, videoURL); err != nil {
				logger.With(zap.Error(err)).Error("Send post videos")
				continue
			}
		}
	}

	return nil
}
