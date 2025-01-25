package matcher

import (
	"context"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/redbook"
	"go.uber.org/zap"
)

type RedbookPostMatcher struct {
	GenericMatcher

	api *redbook.API
}

func NewRedbookPostMatcher(cfg *config.Config, s *discord.Session) (Matcher, error) {
	api, err := redbook.NewAPI(cfg.Redbook)
	if err != nil {
		return nil, err
	}

	return &RedbookPostMatcher{
		api: api,
	}, nil
}

func (m *RedbookPostMatcher) Handle(ctx context.Context, s *discord.MessageSession, matches []string) {
	for _, url := range matches {
		logger := s.Logger.With(
			zap.String("url", url),
		)

		post, err := m.api.GetPost(url)
		if err != nil {
			logger.With(zap.Error(err)).Error("Get post")
			continue
		}

		embeds := post.GetEmbeds()

		s.SendEmbeds(embeds)
		s.SendVideoURLs(post.VideoURLs, post.ID)
	}
}
