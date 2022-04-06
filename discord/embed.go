package discord

import "github.com/bwmarrin/discordgo"

type UpdatableMessageEmbed struct {
	*discordgo.MessageEmbed
	*MessageSession

	idx int
}

func NewUpdatableMessageEmbed(s *Session, m *discordgo.Message) *UpdatableMessageEmbed {
	return &UpdatableMessageEmbed{
		MessageEmbed: m.Embeds[0],

		MessageSession: s.WithMessage(m),
		idx:            0,
	}
}

func (e *UpdatableMessageEmbed) Update() error {
	oldMsg, err := e.MessageSession.ChannelMessage(e.MessageSession.ChannelID, e.MessageSession.Message.ID)
	if err != nil {
		return err
	}

	if e.idx >= len(oldMsg.Embeds) {
		return nil
	}

	newEmbeds := oldMsg.Embeds
	newEmbeds[e.idx] = e.MessageEmbed

	_, err = e.MessageSession.ChannelMessageEditEmbeds(e.MessageSession.ChannelID, e.MessageSession.Message.ID, newEmbeds)
	return err
}

type UpdatableMessageEmbeds []*UpdatableMessageEmbed

func NewUpdatableMessageEmbeds(s *Session, m *discordgo.Message) UpdatableMessageEmbeds {
	ret := make(UpdatableMessageEmbeds, 0, len(m.Embeds))
	for i, embed := range m.Embeds {
		ret = append(ret, &UpdatableMessageEmbed{
			MessageEmbed: embed,

			MessageSession: s.WithMessage(m),
			idx:            i,
		})
	}
	return ret
}

func (es UpdatableMessageEmbeds) Update() error {
	_, err := es[0].MessageSession.ChannelMessageEditEmbeds(es[0].MessageSession.ChannelID, es[0].MessageSession.Message.ID, es.GetRawEmbeds())
	return err
}

func (es UpdatableMessageEmbeds) GetRawEmbeds() []*discordgo.MessageEmbed {
	ret := make([]*discordgo.MessageEmbed, 0, len(es))
	for _, embed := range es {
		ret = append(ret, embed.MessageEmbed)
	}
	return ret
}
