package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func GetNamedLink(text string, url string) string {
	return fmt.Sprintf("[%s](%s)", text, url)
}

func GetMessageMaxBytes(boostTier discordgo.PremiumTier) int64 {
	megaByte := 1000 * 1000
	slackBytes := 5000

	var availableBytes int

	switch boostTier {
	case discordgo.PremiumTier3:
		availableBytes = 100 * megaByte
	case discordgo.PremiumTier2:
		availableBytes = 50 * megaByte
	default:
		availableBytes = 8 * megaByte
	}

	return int64(availableBytes - slackBytes)
}
