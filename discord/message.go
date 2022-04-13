package discord

import "github.com/bwmarrin/discordgo"

type UpdatableMessage struct {
	*discordgo.Message
	*MessageSession
}

func NewUpdatableMessage(s *Session, m *discordgo.Message) *UpdatableMessage {
	return &UpdatableMessage{
		Message:        m,
		MessageSession: s.WithMessage(m),
	}
}

func (m *UpdatableMessage) Update() error {
	_, err := m.MessageSession.ChannelMessageEdit(m.ChannelID, m.ID, m.Message.Content)
	return err
}
