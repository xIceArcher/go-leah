package handler

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
)

type DiscordBotMessageHandlerFunc func(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, matches []string) error

func (handler DiscordBotMessageHandlerFunc) GetHandlerName() string {
	fullName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	tokenizedName := strings.Split(fullName, ".")
	funcName := tokenizedName[len(tokenizedName)-1]
	return strings.ToLower(funcName[0:1]) + funcName[1:]
}

var implementedHandlers []DiscordBotMessageHandlerFunc = []DiscordBotMessageHandlerFunc{
	YoutubeLiveStream,
	InstagramPost,
}

type DiscordBotMessageHandler struct {
	Name        string
	Regexs      []*regexp.Regexp
	HandlerFunc DiscordBotMessageHandlerFunc
}

func GetHandlers(toLoad map[string]*config.DiscordHandlerConfig) (handlers []*DiscordBotMessageHandler, err error) {
	availableHandlers := make(map[string]DiscordBotMessageHandlerFunc)
	for _, handler := range implementedHandlers {
		availableHandlers[handler.GetHandlerName()] = handler
	}

	for name, cfg := range toLoad {
		handler, ok := availableHandlers[name]
		if !ok {
			return nil, fmt.Errorf("handler %s not found", name)
		}

		if len(cfg.Regexes) == 0 {
			return nil, fmt.Errorf("handler %s has no regexes", name)
		}

		regexes := make([]*regexp.Regexp, 0, len(cfg.Regexes))
		for _, regexStr := range cfg.Regexes {
			regex, err := regexp.Compile(regexStr)
			if err != nil {
				return nil, fmt.Errorf("regex %s is invalid", regexStr)
			}

			regexes = append(regexes, regex)
		}

		handlers = append(handlers, &DiscordBotMessageHandler{
			Name:        name,
			Regexs:      regexes,
			HandlerFunc: handler,
		})
	}

	return handlers, nil
}
