package utils

import "fmt"

func GetDiscordNamedLink(text string, url string) string {
	return fmt.Sprintf("[%s](%s)", text, url)
}
