package twitch

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/utils"
)

func (s *Stream) GetEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:   s.URL(),
		Title: s.Title,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: s.ThumbnailURL,
		},
		Author:    s.User.GetEmbed(),
		Timestamp: s.StartedAt.Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Started",
				Value:  utils.FormatDiscordRelativeTime(s.StartedAt),
				Inline: true,
			},
			{
				Name:   "Viewers",
				Value:  fmt.Sprint(s.ViewerCount),
				Inline: true,
			},
		},
		Color: utils.ParseHexColor(consts.ColorTwitch),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "Twitch",
			IconURL: "https://cdn4.iconfinder.com/data/icons/logos-and-brands/512/343_Twitch_logo-512.png",
		},
	}
}

func (u *User) GetEmbed() *discordgo.MessageEmbedAuthor {
	return &discordgo.MessageEmbedAuthor{
		Name:    u.Name,
		URL:     u.URL(),
		IconURL: u.ProfileImageURL,
	}
}
