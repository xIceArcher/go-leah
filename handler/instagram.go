package handler

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/instagram"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

type InstagramPostHandler struct {
	api *instagram.API

	RegexManager
	unimplementedHandler
}

func (InstagramPostHandler) String() string {
	return "instagramPost"
}

func (h *InstagramPostHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp, wg *sync.WaitGroup) (err error) {
	h.Regexes = regexes
	h.api, err = instagram.NewAPI(cfg.Instagram)
	return err
}

func (h *InstagramPostHandler) Handle(ctx context.Context, session *discordgo.Session, guildID string, channelID string, msg string, logger *zap.SugaredLogger) (shortcodes []string, err error) {
	shortcodes = h.Match(msg)

	for _, shortcode := range shortcodes {
		logger := logger.With(
			zap.String("shortcode", shortcode),
		)

		post, err := h.api.GetPost(shortcode)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get post")
			continue
		}

		textWithEntities := &utils.TextWithEntities{Text: post.Text}
		textWithEntities.AddByRegex(instagram.MentionRegex, func(s string) string {
			return utils.GetDiscordNamedLink(s, h.api.GetMentionURL(s))
		})
		textWithEntities.AddByRegex(instagram.HashtagRegex, func(s string) string {
			return utils.GetDiscordNamedLink(s, h.api.GetHashtagURL(s))
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

		if len(post.PhotoURLs) > 0 {
			embeds[len(embeds)-1].Image = &discordgo.MessageEmbedImage{
				URL: post.PhotoURLs[0],
			}
		}

		embeds[len(embeds)-1].Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Likes",
				Value:  fmt.Sprintf("%v", post.Likes),
				Inline: true,
			},
		}

		if len(post.PhotoURLs) > 0 {
			for _, photoURL := range post.PhotoURLs[1:] {
				embeds = append(embeds, &discordgo.MessageEmbed{
					Image: &discordgo.MessageEmbedImage{
						URL: photoURL,
					},
					Color: utils.ParseHexColor(consts.ColorInsta),
				})
			}
		}

		embeds[len(embeds)-1].Footer = &discordgo.MessageEmbedFooter{
			Text:    "Instagram",
			IconURL: "https://instagram-brand.com/wp-content/uploads/2016/11/Instagram_AppIcon_Aug2017.png?w=300",
		}
		embeds[len(embeds)-1].Timestamp = post.Timestamp.Format(time.RFC3339)

		if len(embeds) > 10 {
			logger.Warn("More than 10 embeds in one message")
		}

		maxMessageBytes := utils.GetDiscordMessageMaxBytes(discordgo.PremiumTierNone)

		guild, err := session.Guild(guildID)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get guild")
		} else {
			maxMessageBytes = utils.GetDiscordMessageMaxBytes(guild.PremiumTier)
		}

		videoMessages := utils.SplitVideos(post.VideoURLs, shortcode, maxMessageBytes)

		if _, err = session.ChannelMessageSendEmbeds(channelID, embeds); err != nil {
			logger.With(zap.Error(err)).Error("Send post embeds")
			continue
		}

		for _, message := range videoMessages {
			if _, err = session.ChannelMessageSendComplex(channelID, message); err != nil {
				logger.With(zap.Error(err)).Error("Send post videos")
				continue
			}
		}
	}

	return shortcodes, nil
}

type InstagramStoryHandler struct {
	api *instagram.API

	RegexManager
	unimplementedHandler
}

func (InstagramStoryHandler) String() string {
	return "instagramStory"
}

func (h *InstagramStoryHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp, wg *sync.WaitGroup) (err error) {
	h.Regexes = regexes
	h.api, err = instagram.NewAPI(cfg.Instagram)
	return err
}

func (h *InstagramStoryHandler) Handle(ctx context.Context, session *discordgo.Session, guildID string, channelID string, msg string, logger *zap.SugaredLogger) (matches []string, err error) {
	matches = h.Match(msg)

	for _, match := range matches {
		logger := logger.With(
			zap.String("match", match),
		)

		username := match
		storyID := ""
		if strings.Contains(match, "/") {
			parts := strings.Split(match, "/")
			if len(parts) != 2 {
				logger.Error("Unknown match")
				continue
			}

			username, storyID = parts[0], parts[1]
		}

		story, err := h.api.GetStory(username, storyID)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get story")
			continue
		}

		embed := &discordgo.MessageEmbed{
			URL:   story.URL(),
			Title: fmt.Sprintf("Instagram story by %s", story.Owner.Fullname),
			Author: &discordgo.MessageEmbedAuthor{
				Name:    fmt.Sprintf("%s (%s)", story.Owner.Fullname, story.Owner.Username),
				URL:     story.Owner.URL(),
				IconURL: story.Owner.ProfilePicURL,
			},
			Color: utils.ParseHexColor(consts.ColorInsta),
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "Instagram",
				IconURL: "https://instagram-brand.com/wp-content/uploads/2016/11/Instagram_AppIcon_Aug2017.png?w=300",
			},
			Timestamp: story.Timestamp.Format(time.RFC3339),
		}

		baseMessage := &discordgo.MessageSend{
			Embed: embed,
		}

		maxMessageBytes := utils.GetDiscordMessageMaxBytes(discordgo.PremiumTierNone)

		guild, err := session.Guild(guildID)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get guild")
		} else {
			maxMessageBytes = utils.GetDiscordMessageMaxBytes(guild.PremiumTier)
		}

		if story.IsImage() {
			utils.DownloadAndAttachImage(baseMessage, story.MediaURL, username, maxMessageBytes)
		}

		if _, err = session.ChannelMessageSendComplex(channelID, baseMessage); err != nil {
			logger.With(zap.Error(err)).Error("Send story embed")
			continue
		}

		if story.IsVideo() {
			videoMessage := utils.DownloadVideo(story.MediaURL, match, maxMessageBytes)
			if _, err = session.ChannelMessageSendComplex(channelID, videoMessage); err != nil {
				logger.With(zap.Error(err)).Error("Send post video")
				continue
			}
		}
	}

	return matches, nil
}
