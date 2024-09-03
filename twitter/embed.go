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

func (t *Tweet) GetEmbeds() []*discordgo.MessageEmbed {
	var mainEmbed *discordgo.MessageEmbed
	var relevantPhotos []*Photo

	if t.IsReply {
		mainEmbed = t.replyMainEmbed()
		relevantPhotos = t.Photos
	} else if t.IsRetweet {
		mainEmbed = t.retweetMainEmbed()
		relevantPhotos = t.RetweetedStatus.Photos
	} else if t.IsQuoted {
		mainEmbed = t.quotedMainEmbed()

		if len(t.Photos) > 0 {
			relevantPhotos = t.Photos
		} else if len(t.QuotedStatus.Photos) > 0 {
			relevantPhotos = t.QuotedStatus.Photos
		}
	} else {
		mainEmbed = t.standardMainEmbed()
		relevantPhotos = t.Photos
	}

	altTextField := &discordgo.MessageEmbedField{
		Name: "Alt Text",
	}

	otherEmbeds := make([]*discordgo.MessageEmbed, 0)
	for i, photo := range relevantPhotos {
		if i == 0 {
			mainEmbed.Image = &discordgo.MessageEmbedImage{
				URL: photo.URL,
			}
		} else {
			photoEmbed := photo.GetEmbed()
			photoEmbed.URL = t.URL()

			otherEmbeds = append(otherEmbeds, photoEmbed)
		}

		if photo.AltText != "" {
			altTextField.Value += discord.GetNamedLink(fmt.Sprintf("Image %v", i+1), photo.URL) + "\n" + photo.AltText + "\n\n"
		}
	}

	altTextField.Value = strings.TrimSpace(altTextField.Value)
	if altTextField.Value != "" {
		mainEmbed.Fields = append(mainEmbed.Fields, altTextField)
	}

	mainEmbed.Author = t.User.GetEmbed()

	mainEmbed.Footer = twitterEmbedFooter
	if t.Timestamp.Unix() != 0 {
		mainEmbed.Timestamp = t.Timestamp.Format(time.RFC3339)
	}

	embeds := []*discordgo.MessageEmbed{mainEmbed}
	embeds = append(embeds, otherEmbeds...)
	return embeds
}

func (t *Tweet) standardMainEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:         t.URL(),
		Title:       fmt.Sprintf("Tweet by %s", t.User.Name),
		Description: t.GetTextWithEmbeds(),
		Color:       utils.ParseHexColor(consts.ColorTwitter),
	}
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

	return embed
}

func (t *Tweet) replyMainEmbed() *discordgo.MessageEmbed {
	embed := t.standardMainEmbed()
	embed.Title = fmt.Sprintf("Reply to %s (@%s)", t.ReplyUser.Name, t.ReplyUser.ScreenName)
	return embed
}

func (t *Tweet) GetPhotoEmbeds() []*discordgo.MessageEmbed {
	embeds := make([]*discordgo.MessageEmbed, 0, len(t.Photos))
	for _, photo := range t.Photos {
		photoEmbed := photo.GetEmbed()
		photoEmbed.URL = t.URL()

		embeds = append(embeds, photoEmbed)
	}

	return embeds
}

func (t *Tweet) GetTextWithEmbeds() string {
	if t.IsRetweet {
		return t.RetweetedStatus.GetTextWithEmbeds()
	}
	return strings.TrimSpace(t.Text)
}

func (p *Photo) GetEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Image: &discordgo.MessageEmbedImage{
			URL: p.URL,
		},
		Color: utils.ParseHexColor(consts.ColorTwitter),
	}
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
