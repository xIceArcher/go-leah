package utils

import "github.com/bwmarrin/discordgo"

type UpdatableEmbed struct {
	*discordgo.MessageEmbed

	idx       int
	session   *discordgo.Session
	channelID string
	messageID string
}

func NewEmbed(s *discordgo.Session, embed *discordgo.MessageEmbed) *UpdatableEmbed {
	return &UpdatableEmbed{
		MessageEmbed: embed,
		session:      s,
	}
}

func (e *UpdatableEmbed) Send(channelID string) error {
	m, err := e.session.ChannelMessageSendEmbed(channelID, e.MessageEmbed)
	if err != nil {
		return err
	}

	e.channelID = m.ChannelID
	e.messageID = m.ID
	return nil
}

func (e *UpdatableEmbed) Update() error {
	msg, err := e.session.ChannelMessage(e.channelID, e.messageID)
	if err != nil {
		return err
	}

	if e.idx >= len(msg.Embeds) {
		return nil
	}

	msg.Embeds[e.idx] = e.MessageEmbed

	_, err = e.session.ChannelMessageEditEmbeds(e.channelID, e.messageID, msg.Embeds)
	return err
}

type UpdatableEmbeds struct {
	Embeds []*UpdatableEmbed

	session   *discordgo.Session
	channelID string
	messageID string
}

func NewEmbeds(s *discordgo.Session, embeds []*discordgo.MessageEmbed) *UpdatableEmbeds {
	updatableEmbeds := make([]*UpdatableEmbed, 0, len(embeds))
	for i, embed := range embeds {
		updatableEmbed := NewEmbed(s, embed)
		updatableEmbed.idx = i

		updatableEmbeds = append(updatableEmbeds, updatableEmbed)
	}

	return &UpdatableEmbeds{
		session: s,
		Embeds:  updatableEmbeds,
	}
}

func (e *UpdatableEmbeds) Send(channelID string) error {
	if len(e.Embeds) == 0 {
		return nil
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(e.Embeds))
	for _, embed := range e.Embeds {
		embeds = append(embeds, embed.MessageEmbed)
	}

	m, err := e.session.ChannelMessageSendEmbeds(channelID, embeds)
	if err != nil {
		return err
	}

	e.channelID = m.ChannelID
	e.messageID = m.ID

	for _, embed := range e.Embeds {
		embed.channelID = m.ChannelID
		embed.messageID = m.ID
	}

	return nil
}

func (e *UpdatableEmbeds) Update() error {
	embeds := make([]*discordgo.MessageEmbed, 0, len(e.Embeds))
	for _, embed := range e.Embeds {
		embeds = append(embeds, embed.MessageEmbed)
	}

	_, err := e.session.ChannelMessageEditEmbeds(e.channelID, e.messageID, embeds)
	return err
}
