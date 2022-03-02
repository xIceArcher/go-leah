package tiktok

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/utils"
)

func (v *Video) GetDiscordMessages(maxMessageBytes int64) []*discordgo.MessageSend {
	messages := make([]*discordgo.MessageSend, 0, 2)

	textWithEntities := &utils.TextWithEntities{Text: v.Description}
	textWithEntities.AddEntities(func(u *utils.Entity) string {
		return utils.GetDiscordNamedLink(u.Match, GetTagURL(u.Match[1:]))
	}, v.Tags...)
	textWithEntities.AddEntities(func(u *utils.Entity) string {
		return utils.GetDiscordNamedLink(u.Match, GetMentionURL(u.Match[1:]))
	}, v.Mentions...)

	segmentedText := textWithEntities.GetReplacedText(4096, 1)

	embed := &discordgo.MessageEmbed{
		URL:   v.URL(),
		Title: fmt.Sprintf("Video by %s", v.Author.Nickname),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (@%s)", v.Author.Nickname, v.Author.UniqueID),
			URL:     v.Author.URL(),
			IconURL: v.Author.AvatarURL,
		},
		Description: segmentedText[0],
		Color:       utils.ParseHexColor(consts.ColorTiktok),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Music",
				Value: utils.GetDiscordNamedLink(v.Music.String(), v.Music.URL()),
			},
			{
				Name:   "Likes",
				Value:  strconv.FormatUint(v.LikeCount, 10),
				Inline: true,
			},
			{
				Name:   "Comments",
				Value:  strconv.FormatUint(v.CommentCount, 10),
				Inline: true,
			},
			{
				Name:   "Shares",
				Value:  strconv.FormatUint(v.ShareCount, 10),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "Tiktok",
			IconURL: "https://cdn4.iconfinder.com/data/icons/social-media-flat-7/64/Social-media_Tiktok-512.png",
		},
		Timestamp: v.CreateTime.Format(time.RFC3339),
	}

	messages = append(messages, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	messages = append(messages, utils.DownloadVideo(v.VideoURL, v.ID, maxMessageBytes))

	return messages
}
