package handler

import (
	"context"
	"fmt"
	"regexp"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/matcher"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

type RegexHandler struct {
	GenericHandler
	FilterRegexes []*regexp.Regexp

	Matchers []*MatcherWithRegexes
}

type MatcherWithRegexes struct {
	Name string

	matcher.Matcher
	Regexes []*regexp.Regexp
}

func NewRegexHandler(cfg *config.Config, s *discord.Session) (MessageHandler, error) {
	filterRegexes := make([]*regexp.Regexp, 0, len(cfg.Discord.FilterRegexes))
	for _, regexStr := range cfg.Discord.FilterRegexes {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("regex %s is invalid", regexStr)
		}

		filterRegexes = append(filterRegexes, regex)
	}

	implementedMatchers := map[string]matcher.Constructor{
		"youtubeLiveStream": matcher.NewYoutubeLiveStreamMatcher,
		"instagramPost":     matcher.NewInstagramPostMatcher,
		"instagramStory":    matcher.NewInstagramStoryMatcher,
		"twitchLiveStream":  matcher.NewTwitchLiveStreamMatcher,
		"twitterPost":       matcher.NewTwitterPostMatcher,
		"tiktokVideo":       matcher.NewTiktokVideoMatcher,
		"redbookPost":       matcher.NewRedbookPostMatcher,
	}

	matchersWithRegexes := make([]*MatcherWithRegexes, 0, len(implementedMatchers))
	for matcherName, matcherConfig := range cfg.Discord.Handlers {
		matcherConstructor, ok := implementedMatchers[matcherName]
		if !ok {
			return nil, fmt.Errorf("matcher %s not found", matcherName)
		}

		if len(matcherConfig.Regexes) == 0 {
			return nil, fmt.Errorf("matcher %s has no regexes", matcherName)
		}

		regexes := make([]*regexp.Regexp, 0, len(matcherConfig.Regexes))
		for _, regexStr := range matcherConfig.Regexes {
			regex, err := regexp.Compile(regexStr)
			if err != nil {
				return nil, fmt.Errorf("regex %s in handler %s is invalid", regexStr, matcherName)
			}

			regexes = append(regexes, regex)
		}

		logger := s.Logger.With(zap.String("matcher", matcherName))

		logger.Info("Initializing matcher...")
		m, err := matcherConstructor(cfg, s)
		if err != nil {
			return nil, err
		}
		logger.Info("Initialized matcher")

		matchersWithRegexes = append(matchersWithRegexes, &MatcherWithRegexes{
			Name:    matcherName,
			Matcher: m,
			Regexes: regexes,
		})
	}

	return &RegexHandler{
		FilterRegexes: filterRegexes,
		Matchers:      matchersWithRegexes,
	}, nil
}

func (h *RegexHandler) Handle(ctx context.Context, s *discord.MessageSession) bool {
	for _, regex := range h.FilterRegexes {
		s.Content = regex.ReplaceAllLiteralString(s.Content, "")
	}

	matched := false

	for _, matcher := range h.Matchers {
		matches := make([]string, 0)

		for _, regex := range matcher.Regexes {
			currMatches := regex.FindAllStringSubmatch(s.Content, -1)
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

		if len(matches) > 0 {
			matched = true
			matches = utils.Unique(matches)

			s.Logger = s.Logger.With(
				zap.String("matcher", matcher.Name),
				zap.Strings("matches", matches),
			)

			matcher.Handle(ctx, s, matches)
			s.Logger.Info("Success")
		}
	}

	return matched
}

func (h *RegexHandler) Stop() {
	for _, matcher := range h.Matchers {
		matcher.Stop()
	}
}
