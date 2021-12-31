package handler

import (
	"context"
	"regexp"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/twitter"
	"go.uber.org/zap"
)

type TwitterSpaceHandler struct {
	api twitter.API

	RegexManager
}

func (TwitterSpaceHandler) String() string {
	return "twitterSpace"
}

func (h *TwitterSpaceHandler) Setup(ctx context.Context, cfg *config.Config, regexes []*regexp.Regexp, wg *sync.WaitGroup) (err error) {
	h.Regexes = regexes
	h.api = twitter.NewBaseAPI(cfg.Twitter)
	return err
}

func (h *TwitterSpaceHandler) Resume(ctx context.Context, session *discordgo.Session, logger *zap.SugaredLogger) {
}

func (h *TwitterSpaceHandler) Handle(ctx context.Context, session *discordgo.Session, channelID string, msg string, logger *zap.SugaredLogger) (spaceIDs []string, err error) {
	spaceIDs = h.Match(msg)
	embeds := make([]*discordgo.MessageEmbed, 0, len(spaceIDs))

	for _, spaceID := range spaceIDs {
		if len(embeds) == 10 {
			logger.Warn("More than 10 embeds in one message")
			break
		}

		space, err := h.api.GetSpace(spaceID)
		if err != nil {
			continue
		}

		embeds = append(embeds, space.GetEmbed())
	}

	if len(embeds) > 0 {
		if _, err := session.ChannelMessageSendEmbeds(channelID, embeds); err != nil {
			return nil, err
		}
	}

	return spaceIDs, nil
}
