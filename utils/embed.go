package utils

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrEmbedsNotFound error = fmt.Errorf("message has no embeds")
)

type UpdatableEmbed struct {
	*discordgo.MessageEmbed

	Idx       int
	ChannelID string
	MessageID string

	session *discordgo.Session
}

func NewEmbed(s *discordgo.Session, embed *discordgo.MessageEmbed) *UpdatableEmbed {
	return &UpdatableEmbed{
		MessageEmbed: embed,
		session:      s,
	}
}

func LoadEmbed(s *discordgo.Session, channelID string, messageID string, idx int) (*UpdatableEmbed, error) {
	msg, err := s.ChannelMessage(channelID, messageID)
	if err != nil {
		return nil, err
	}

	if idx >= len(msg.Embeds) {
		return nil, ErrEmbedsNotFound
	}

	return &UpdatableEmbed{
		MessageEmbed: msg.Embeds[idx],
		Idx:          idx,
		ChannelID:    channelID,
		MessageID:    messageID,

		session: s,
	}, nil
}

func (e *UpdatableEmbed) Send(channelID string) error {
	m, err := e.session.ChannelMessageSendEmbed(channelID, e.MessageEmbed)
	if err != nil {
		return err
	}

	e.ChannelID = m.ChannelID
	e.MessageID = m.ID
	return nil
}

func (e *UpdatableEmbed) Update() error {
	msg, err := e.session.ChannelMessage(e.ChannelID, e.MessageID)
	if err != nil {
		return err
	}

	if e.Idx >= len(msg.Embeds) {
		return nil
	}

	msg.Embeds[e.Idx] = e.MessageEmbed

	_, err = e.session.ChannelMessageEditEmbeds(e.ChannelID, e.MessageID, msg.Embeds)
	return err
}

type UpdatableEmbeds struct {
	Embeds []*UpdatableEmbed

	session   *discordgo.Session
	ChannelID string
	MessageID string
}

func NewEmbeds(s *discordgo.Session, embeds []*discordgo.MessageEmbed) *UpdatableEmbeds {
	updatableEmbeds := make([]*UpdatableEmbed, 0, len(embeds))
	for i, embed := range embeds {
		updatableEmbed := NewEmbed(s, embed)
		updatableEmbed.Idx = i

		updatableEmbeds = append(updatableEmbeds, updatableEmbed)
	}

	return &UpdatableEmbeds{
		session: s,
		Embeds:  updatableEmbeds,
	}
}

func LoadEmbeds(s *discordgo.Session, channelID string, messageID string) (*UpdatableEmbeds, error) {
	msg, err := s.ChannelMessage(channelID, messageID)
	if err != nil {
		return nil, err
	}

	if len(msg.Embeds) == 0 {
		return nil, ErrEmbedsNotFound
	}

	return NewEmbeds(s, msg.Embeds), nil
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

	e.ChannelID = m.ChannelID
	e.MessageID = m.ID

	for _, embed := range e.Embeds {
		embed.ChannelID = m.ChannelID
		embed.MessageID = m.ID
	}

	return nil
}

func (e *UpdatableEmbeds) Update() error {
	embeds := make([]*discordgo.MessageEmbed, 0, len(e.Embeds))
	for _, embed := range e.Embeds {
		embeds = append(embeds, embed.MessageEmbed)
	}

	_, err := e.session.ChannelMessageEditEmbeds(e.ChannelID, e.MessageID, embeds)
	return err
}
