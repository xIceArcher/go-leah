package redbook

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/utils"
)

const (
	MAX_EMBEDS_PER_POST = 4
)

var (
	redbookFooter = &discordgo.MessageEmbedFooter{
		Text:    "Redbook",
		IconURL: "https://assets.stickpng.com/images/5c77b61f003fa702a1d27933.png",
	}
)

func (p *Post) GetEmbeds() (embeds []*discordgo.MessageEmbed) {
	embeds = append(embeds, &discordgo.MessageEmbed{
		URL:         p.URL,
		Title:       p.Title,
		Description: p.Description,
		Color:       utils.ParseHexColor(consts.ColorRedbook),
		Author: &discordgo.MessageEmbedAuthor{
			URL:  p.Author.URL,
			Name: p.Author.Name,
		},
	})

	footerEmbedIdx := len(embeds) - 1
	for i, photoURL := range p.PhotoURLs {
		if i == 0 {
			embeds[len(embeds)-1].Image = &discordgo.MessageEmbedImage{
				URL: p.PhotoURLs[0],
			}
		} else {
			embedURL := p.URL
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

	embeds[footerEmbedIdx].Footer = redbookFooter
	embeds[footerEmbedIdx].Timestamp = p.CreateTime.Format(time.RFC3339)

	return embeds
}
