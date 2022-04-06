package matcher

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/twitch"
	"go.uber.org/zap"
)

type TwitchLiveStreamMatcher struct {
	GenericMatcher

	api *twitch.API
}

func NewTwitchLiveStreamMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	api, err := twitch.NewAPI(cfg.Twitch)
	if err != nil {
		return nil, err
	}

	return &TwitchLiveStreamMatcher{
		api: api,
	}, nil
}

func (m *TwitchLiveStreamMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	embeds := make([]*discordgo.MessageEmbed, 0, len(matches))

	for _, loginName := range matches {
		logger := s.Logger.With(
			zap.String("loginName", loginName),
		)

		streamInfo, err := m.api.GetStream(loginName)
		if errors.Is(err, twitch.ErrNotFound) {
			logger.Info("Stream is not found or not live")
			continue
		} else if err != nil {
			logger.With(zap.Error(err)).Error("Failed to get stream")
			continue
		}

		embeds = append(embeds, streamInfo.GetEmbed())
	}

	s.SendEmbeds(embeds)
}
