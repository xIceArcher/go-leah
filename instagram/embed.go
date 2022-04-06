package instagram

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/utils"
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
			Description: text,
			Color:       utils.ParseHexColor(consts.ColorInsta),
		})
	}

	if len(p.PhotoURLs) > 0 {
		embeds[len(embeds)-1].Image = &discordgo.MessageEmbedImage{
			URL: p.PhotoURLs[0],
		}
	}

	embeds[len(embeds)-1].Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "Likes",
			Value:  fmt.Sprintf("%v", p.Likes),
			Inline: true,
		},
	}

	if len(p.PhotoURLs) > 0 {
		for _, photoURL := range p.PhotoURLs[1:] {
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
	embeds[len(embeds)-1].Timestamp = p.Timestamp.Format(time.RFC3339)

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
