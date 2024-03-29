package twitter

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/utils"
)

var twitterEmbedFooter = &discordgo.MessageEmbedFooter{
	Text:    "Twitter",
	IconURL: "https://abs.twimg.com/icons/apple-touch-icon-192x192.png",
}

func (t *Tweet) GetEmbeds() (embeds []*discordgo.MessageEmbed) {
	if t.IsReply {
		embeds = append(embeds, t.replyMainEmbed())
	} else if t.IsRetweet {
		embeds = append(embeds, t.retweetMainEmbed())
	} else if t.IsQuoted {
		embeds = append(embeds, t.quotedMainEmbed())
	} else {
		embeds = append(embeds, t.standardMainEmbed())
	}

	embeds[0].Author = t.User.GetEmbed()

	if t.IsRetweet && len(t.RetweetedStatus.PhotoURLs) > 1 {
		embeds = append(embeds, t.RetweetedStatus.GetPhotoEmbeds()[1:]...)
	} else if len(t.PhotoURLs) > 1 {
		embeds = append(embeds, t.GetPhotoEmbeds()[1:]...)
	}

	embeds[0].Footer = twitterEmbedFooter
	if t.Timestamp.Unix() != 0 {
		embeds[0].Timestamp = t.Timestamp.Format(time.RFC3339)
	}
	return embeds
}

func (t *Tweet) standardMainEmbed() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		URL:         t.URL(),
		Title:       fmt.Sprintf("Tweet by %s", t.User.Name),
		Description: t.GetTextWithEmbeds(),
		Color:       utils.ParseHexColor(consts.ColorTwitter),
	}

	if len(t.PhotoURLs) != 0 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: t.PhotoURLs[0],
		}
	}

	return embed
}

func (t *Tweet) retweetMainEmbed() *discordgo.MessageEmbed {
	embed := t.RetweetedStatus.GetEmbeds()[0]
	embed.Title = fmt.Sprintf("Retweeted %s (@%s)", t.RetweetedStatus.User.Name, t.RetweetedStatus.User.ScreenName)

	return embed
}

func (t *Tweet) quotedMainEmbed() *discordgo.MessageEmbed {
	embed := t.standardMainEmbed()

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name: "Quote",
			Value: discord.GetNamedLink(
				fmt.Sprintf("Quoted tweet by %s (@%s)", t.QuotedStatus.User.Name, t.QuotedStatus.User.ScreenName),
				t.QuotedStatus.URL(),
			) + "\n" + t.QuotedStatus.GetTextWithEmbeds(),
		},
	}

	if embed.Image == nil {
		if len(t.QuotedStatus.PhotoURLs) != 0 {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: t.QuotedStatus.PhotoURLs[0],
			}
		}
	}

	return embed
}

func (t *Tweet) replyMainEmbed() *discordgo.MessageEmbed {
	embed := t.standardMainEmbed()
	embed.Title = fmt.Sprintf("Reply to %s (@%s)", t.ReplyUser.Name, t.ReplyUser.ScreenName)
	return embed
}

func (t *Tweet) GetPhotoEmbeds() []*discordgo.MessageEmbed {
	embeds := make([]*discordgo.MessageEmbed, 0, len(t.PhotoURLs))
	for _, url := range t.PhotoURLs {
		embeds = append(embeds, &discordgo.MessageEmbed{
			URL: t.URL(),
			Image: &discordgo.MessageEmbedImage{
				URL: url,
			},
			Color: utils.ParseHexColor(consts.ColorTwitter),
		})
	}

	return embeds
}

func (t *Tweet) GetTextWithEmbeds() string {
	if t.IsRetweet {
		return t.RetweetedStatus.GetTextWithEmbeds()
	}
	return strings.TrimSpace(t.Text)
}

func (u *User) GetEmbed() *discordgo.MessageEmbedAuthor {
	return &discordgo.MessageEmbedAuthor{
		Name:    fmt.Sprintf("%s (@%s)", u.Name, u.ScreenName),
		URL:     u.URL(),
		IconURL: u.ProfileImageURL,
	}
}

func (s *Space) GetEmbed() *discordgo.MessageEmbed {
	var color string
	fields := make([]*discordgo.MessageEmbedField, 0)

	if s.State == SpaceStateLive {
		color = consts.ColorTwitter
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Started",
			Value:  utils.FormatDiscordRelativeTime(s.StartTime),
			Inline: true,
		})
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Participants",
			Value:  fmt.Sprint(s.ParticipantCount),
			Inline: true,
		})
	} else {
		color = consts.ColorNone
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Ended",
			Value:  utils.FormatDiscordRelativeTime(s.EndTime),
			Inline: true,
		})
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Duration",
			Value:  utils.FormatDurationSimple(s.EndTime.Sub(s.StartTime)),
			Inline: true,
		})
	}

	return &discordgo.MessageEmbed{
		URL:       s.URL(),
		Title:     s.Title,
		Author:    s.Creator.GetEmbed(),
		Timestamp: s.StartTime.Format(time.RFC3339),
		Fields:    fields,
		Color:     utils.ParseHexColor(color),
		Footer:    twitterEmbedFooter,
	}
}
