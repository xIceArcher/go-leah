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

func DownloadAndAttachImage(message *discordgo.MessageSend, url string, fileName string) {
	file, _, err := Download(url, int64(consts.DiscordMessageMaxBytes))
	if err != nil {
		message.Embed.Image = &discordgo.MessageEmbedImage{
			URL: url,
		}
		return
	}

	message.Files = []*discordgo.File{
		{
			Name:        fmt.Sprintf("%s.jpg", fileName),
			ContentType: "image/jpeg",
			Reader:      file,
		},
	}

	if len(message.Embeds) == 0 {
		if message.Embed != nil {
			message.Embeds = append(message.Embeds, message.Embed)
			message.Embed = nil
		} else {
			message.Embeds = append(message.Embeds, &discordgo.MessageEmbed{})
		}
	}

	message.Embeds[0].Image = &discordgo.MessageEmbedImage{
		URL: fmt.Sprintf("attachment://%s.jpg", fileName),
	}
}

func DownloadVideo(url string, fileName string) *discordgo.MessageSend {
	file, _, err := Download(url, int64(consts.DiscordMessageMaxBytes))
	if err != nil {
		return &discordgo.MessageSend{
			Content: url,
		}
	}

	return &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:        fmt.Sprintf("%s.mp4", fileName),
				ContentType: "video/mp4",
				Reader:      file,
			},
		},
	}
}

func SplitVideos(urls []string, fileNamePrefix string) (messages []*discordgo.MessageSend) {
	maxBytes := int64(consts.DiscordMessageMaxBytes)

	files := make([]*discordgo.File, 0)
	remainingBytes := int64(consts.DiscordMessageMaxBytes)

	for i, url := range urls {
		file, bytes, err := Download(url, remainingBytes)
		if errors.Is(err, ErrResponseTooLong) && bytes < maxBytes {
			// This video can't fit in the current message, but does not exceed the maximum bytes in a single message
			// Flush the current files to a message and start a new message
			// Then redownload the video
			messages = append(messages, &discordgo.MessageSend{
				Files: files,
			})
			files = make([]*discordgo.File, 0)
			remainingBytes = maxBytes

			file, bytes, err = Download(url, remainingBytes)
		}

		if (err != nil && !errors.Is(err, ErrResponseTooLong)) || bytes > maxBytes {
			// Either we can't fetch the link or this video exceeds the maximum bytes in a single message
			// So we just send the video URL
			messages = append(messages, &discordgo.MessageSend{
				Content: url,
			})
			continue
		}

		remainingBytes -= bytes
		files = append(files, &discordgo.File{
			Name:        fmt.Sprintf("%v_%v.mp4", fileNamePrefix, i),
			ContentType: "video/mp4",
			Reader:      file,
		})
	}

	// Flush the last message if there are videos in it
	if len(files) > 0 {
		messages = append(messages, &discordgo.MessageSend{
			Files: files,
		})
	}

	return messages
}
