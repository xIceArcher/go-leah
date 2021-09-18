package utils

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/consts"
)

func GetDiscordNamedLink(text string, url string) string {
	return fmt.Sprintf("[%s](%s)", text, url)
}

func SplitVideos(urls []string, fileNamePrefix string) (messages []*discordgo.MessageSend) {
	maxBytes := int64(consts.DiscordMessageMaxBytes)

	files := make([]*discordgo.File, 0)
	remainingBytes := int64(consts.DiscordMessageMaxBytes)

	for i, url := range urls {
		file, bytes, err := Download(url, remainingBytes)
		if (err != nil && err != ErrResponseTooLong) || bytes > maxBytes {
			// Either we can't fetch the link or this video exceeds the maximum bytes in a single message
			// So we just send the video URL
			messages = append(messages, &discordgo.MessageSend{
				Content: url,
			})
			continue
		}

		if errors.Is(err, ErrResponseTooLong) {
			// This video can't fit in the current message, but does not exceed the maximum bytes in a single message
			// Flush the current files to a message and start a new message
			messages = append(messages, &discordgo.MessageSend{
				Files: files,
			})
			files = make([]*discordgo.File, 0)
			remainingBytes = maxBytes
		}

		remainingBytes -= bytes
		files = append(files, &discordgo.File{
			Name:        fmt.Sprintf("%v_%v.mp4", fileNamePrefix, i),
			ContentType: "video/mp4",
			Reader:      file,
		})
	}

	if len(files) > 0 {
		messages = append(messages, &discordgo.MessageSend{
			Files: files,
		})
	}

	return messages
}
