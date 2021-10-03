package handler

import (
	"context"
	"fmt"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

type MessageHandler interface {
	String() string
	Setup(ctx context.Context, cfg *config.Config, regexp []*regexp.Regexp) error
	Handle(session *discordgo.Session, channelID string, msg string, logger *zap.SugaredLogger) ([]string, error)
}

var implementedHandlers []MessageHandler = []MessageHandler{
	&YoutubeLiveStreamHandler{},
	&InstagramPostHandler{},
	&TwitchLiveStreamHandler{},
}

func SetupHandlers(ctx context.Context, cfg *config.Config, logger *zap.SugaredLogger) (handlers []MessageHandler, err error) {
	availableHandlers := make(map[string]MessageHandler)
	for _, handler := range implementedHandlers {
		availableHandlers[handler.String()] = handler
	}

	for name, handlerCfg := range cfg.Discord.Handlers {
		handler, ok := availableHandlers[name]
		if !ok {
			return nil, fmt.Errorf("handler %s not found", name)
		}

		if len(handlerCfg.Regexes) == 0 {
			return nil, fmt.Errorf("handler %s has no regexes", name)
		}

		regexes := make([]*regexp.Regexp, 0, len(handlerCfg.Regexes))
		for _, regexStr := range handlerCfg.Regexes {
			regex, err := regexp.Compile(regexStr)
			if err != nil {
				return nil, fmt.Errorf("regex %s in handler %s is invalid", regexStr, name)
			}

			regexes = append(regexes, regex)
		}

		if err := handler.Setup(ctx, cfg, regexes); err != nil {
			return nil, err
		}

		handlers = append(handlers, handler)
	}

	logger.Infof("Loaded handlers %s", handlers)
	return handlers, nil
}

type RegexManager struct {
	Regexes []*regexp.Regexp
}

func (r *RegexManager) Match(s string) (matches []string) {
	for _, regex := range r.Regexes {
		currMatches := regex.FindAllStringSubmatch(s, -1)
		for _, match := range currMatches {
			if len(match) > 1 {
				// match[1] is the subgroup
				matches = append(matches, match[1])
			} else {
				// match[0] is the complete match
				matches = append(matches, match[0])
			}
		}
	}

	return utils.Unique(matches)
}
