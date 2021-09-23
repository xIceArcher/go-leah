package twitter

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/utils"
)

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

	embeds[0].Author = &discordgo.MessageEmbedAuthor{
		Name:    fmt.Sprintf("%s (@%s)", t.User.Name, t.User.ScreenName),
		URL:     t.User.URL(),
		IconURL: t.User.ProfileImageURL,
	}

	if len(t.PhotoURLs) > 1 {
		embeds = append(embeds, t.GetPhotoEmbeds()[1:]...)
	}

	embeds[len(embeds)-1].Footer = &discordgo.MessageEmbedFooter{
		Text:    "Twitter",
		IconURL: "https://abs.twimg.com/icons/apple-touch-icon-192x192.png",
	}
	embeds[len(embeds)-1].Timestamp = t.Timestamp.Format(time.RFC3339)

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
	embed := t.standardMainEmbed()
	embed.Title = fmt.Sprintf("Retweeted %s (@%s)", t.RetweetedStatus.User.Name, t.RetweetedStatus.User.ScreenName)

	return embed
}

func (t *Tweet) quotedMainEmbed() *discordgo.MessageEmbed {
	embed := t.standardMainEmbed()

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name: "Quote",
			Value: utils.GetDiscordNamedLink(
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

	textWithEntities := utils.TextWithEntities{Text: t.Text}

	textWithEntities.AddEntity(func(s string) string {
		return utils.GetDiscordNamedLink(s, fmt.Sprintf("https://twitter.com/hashtag/%s", s[1:]))
	}, t.Hashtags...)

	textWithEntities.AddEntity(func(s string) string {
		if t.IsReply && s[1:] == t.ReplyUser.Name {
			return ""
		}
		return utils.GetDiscordNamedLink(s, (&User{Name: s}).URL())
	}, t.UserMentions...)

	textWithEntities.AddEntity(func(s string) string {
		return ""
	}, t.MediaLinks...)

	textWithEntities.AddEntity(func(s string) string {
		finalURL, err := utils.ExpandURL(s, expandIgnoreRegexes...)
		if err != nil {
			return s
		}

		if t.IsQuoted && finalURL == t.QuotedStatus.URL() {
			return ""
		}

		return finalURL
	}, t.URLs...)

	replacedText := textWithEntities.GetReplacedText(4096, -1)

	return strings.TrimSpace(replacedText[0])
}
