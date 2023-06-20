package instagram

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/utils"
)

const (
	MAX_EMBEDS_PER_POST = 4
)

func (p *Post) GetEmbeds() (embeds []*discordgo.MessageEmbed) {
	textWithEntities := &utils.TextWithEntities{Text: p.Text}
	textWithEntities.AddByRegex(MentionRegex, func(s string) string {
		return discord.GetNamedLink(s, (&API{}).GetMentionURL(s))
	})
	textWithEntities.AddByRegex(HashtagRegex, func(s string) string {
		return discord.GetNamedLink(s, (&API{}).GetHashtagURL(s))
	})

	segmentedText := textWithEntities.GetReplacedText(4096, -1)

	embeds = append(embeds, &discordgo.MessageEmbed{
		URL:   p.URL(),
		Title: fmt.Sprintf("Instagram post by %s", p.Owner.Fullname),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (%s)", p.Owner.Fullname, p.Owner.Username),
			URL:     p.Owner.URL(),
			IconURL: p.Owner.ProfilePicURL,
		},
		Description: segmentedText[0],
		Color:       utils.ParseHexColor(consts.ColorInsta),
	})

	for _, text := range segmentedText[1:] {
		embeds = append(embeds, &discordgo.MessageEmbed{
			URL:         p.URL(),
			Description: text,
			Color:       utils.ParseHexColor(consts.ColorInsta),
		})
	}

	footerEmbedIdx := len(embeds) - 1
	for i, photoURL := range p.PhotoURLs {
		if i == 0 {
			embeds[len(embeds)-1].Image = &discordgo.MessageEmbedImage{
				URL: p.PhotoURLs[0],
			}
		} else {
			embedURL := p.URL()
			if i >= MAX_EMBEDS_PER_POST {
				embedURL += fmt.Sprintf("?s=%v", i/MAX_EMBEDS_PER_POST)
			}

			if i%MAX_EMBEDS_PER_POST == 0 {
				footerEmbedIdx += MAX_EMBEDS_PER_POST
			}

			embeds = append(embeds, &discordgo.MessageEmbed{
				URL: embedURL,
				Image: &discordgo.MessageEmbedImage{
					URL: photoURL,
				},
				Color: utils.ParseHexColor(consts.ColorInsta),
			})
		}
	}

	embeds[footerEmbedIdx].Footer = &discordgo.MessageEmbedFooter{
		Text:    "Instagram",
		IconURL: "https://www.instagram.com/static/images/ico/favicon-192.png/68d99ba29cc8.png",
	}
	embeds[footerEmbedIdx].Timestamp = p.Timestamp.Format(time.RFC3339)

	return embeds
}

func (s *Story) GetEmbed() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		URL:   s.URL(),
		Title: fmt.Sprintf("Instagram story by %s", s.Owner.Fullname),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s (%s)", s.Owner.Fullname, s.Owner.Username),
			URL:     s.Owner.URL(),
			IconURL: s.Owner.ProfilePicURL,
		},
		Color: utils.ParseHexColor(consts.ColorInsta),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "Instagram",
			IconURL: "https://instagram-brand.com/wp-content/uploads/2016/11/Instagram_AppIcon_Aug2017.png?w=300",
		},
		Timestamp: s.Timestamp.Format(time.RFC3339),
	}

	if s.IsImage() {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: s.MediaURL,
		}
	}

	return embed
}
