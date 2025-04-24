package discord

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

var (
	ErrMissingPermissions error = fmt.Errorf("missing permissions")
)

type Session struct {
	*discordgo.Session

	Logger *zap.SugaredLogger
}

func NewSession(s *discordgo.Session, l *zap.SugaredLogger) *Session {
	return &Session{
		Session: s,
		Logger:  l,
	}
}

func (s *Session) WithMessage(m *discordgo.Message) *MessageSession {
	return &MessageSession{
		Session: s,
		Message: m,
	}
}

func (s *Session) HasSendMessagePermissions(channelID string) (bool, error) {
	permissions, err := s.State.UserChannelPermissions(s.State.User.ID, channelID)
	if err != nil {
		return false, err
	}

	if permissions&discordgo.PermissionSendMessages == 0 {
		s.Logger.Info("No send message permissions")
		return false, nil
	}

	return true, nil
}

func (s *Session) GetGuildPremiumTier(channelID string) discordgo.PremiumTier {
	channel, err := s.Channel(channelID)
	if err != nil {
		s.Logger.With(zap.Error(err)).Warn("Failed to get channel")
		return discordgo.PremiumTierNone
	}

	guild, err := s.Guild(channel.GuildID)
	if err != nil {
		s.Logger.With(zap.Error(err)).Warn("Failed to get guild")
		return discordgo.PremiumTierNone
	}

	return guild.PremiumTier
}

func (s *Session) GetMessageEmbeds(channelID string, messageID string) (UpdatableMessageEmbeds, error) {
	m, err := s.ChannelMessage(channelID, messageID)
	if err != nil {
		return nil, err
	}

	return NewUpdatableMessageEmbeds(s, m), nil
}

func (s *Session) SendMessage(channelID string, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if msg == "" {
		return
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	if _, err := s.ChannelMessageSend(channelID, msg); err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send message")
	}
}

func (s *Session) SendEmbed(channelID string, embed *discordgo.MessageEmbed) (*UpdatableMessageEmbed, error) {
	if embed == nil {
		return nil, fmt.Errorf("empty embed")
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return nil, ErrMissingPermissions
	}

	m, err := s.ChannelMessageSendEmbed(channelID, processEmbed(embed))
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send embeds")
		return nil, err
	}

	return NewUpdatableMessageEmbed(s, m), nil
}

func (s *Session) DownloadImageAndSendEmbed(channelID string, embed *discordgo.MessageEmbed, fileName string) (*UpdatableMessageEmbed, error) {
	return s.downloadImageAndSendEmbed(channelID, embed, fileName, s.GetGuildPremiumTier(channelID))
}

func (s *Session) downloadImageAndSendEmbed(channelID string, embed *discordgo.MessageEmbed, fileName string, tier discordgo.PremiumTier) (*UpdatableMessageEmbed, error) {
	// Things break when the filename starts with an underscore
	for strings.HasPrefix(fileName, "_") {
		fileName = strings.TrimPrefix(fileName, "_")
	}

	if embed == nil || embed.Image == nil || embed.Image.URL == "" {
		// Embed has no image
		return s.SendEmbed(channelID, embed)
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return nil, ErrMissingPermissions
	}

	file, _, err := utils.Download(embed.Image.URL, GetMessageMaxBytes(tier))
	if err != nil {
		// Image is too big, just send with the URL
		return s.SendEmbed(channelID, embed)
	}

	embed.Image.URL = fmt.Sprintf("attachment://%s.jpg", fileName)

	message := &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{processEmbed(embed)},
		Files: []*discordgo.File{
			{
				Name:        fmt.Sprintf("%s.jpg", fileName),
				ContentType: "image/jpeg",
				Reader:      file,
			},
		},
	}

	m, err := s.ChannelMessageSendComplex(channelID, message)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send complex message")
		return nil, err
	}

	return NewUpdatableMessageEmbed(s, m), nil
}

func (s *Session) SendEmbeds(channelID string, embeds []*discordgo.MessageEmbed) (UpdatableMessageEmbeds, error) {
	if len(embeds) == 0 {
		return UpdatableMessageEmbeds{}, nil
	} else if len(embeds) > 10 {
		s.Logger.Warn("More than 10 embeds in message, only first 10 will be sent...")
		embeds = embeds[:10]
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return nil, ErrMissingPermissions
	}

	m, err := s.ChannelMessageSendEmbeds(channelID, processEmbeds(embeds))
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send embeds")
		return nil, err
	}

	return NewUpdatableMessageEmbeds(s, m), nil
}

func (s *Session) SendVideo(channelID string, video io.ReadCloser, fileName string) {
	s.sendVideo(channelID, video, fileName, s.GetGuildPremiumTier(channelID))
}

func (s *Session) sendVideo(channelID string, video io.ReadCloser, fileName string, tier discordgo.PremiumTier) {
	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	buf := bytes.Buffer{}
	if _, err := buf.ReadFrom(video); err != nil {
		return
	}
	video.Close()

	if buf.Len() > int(GetMessageMaxBytes(tier)) {
		s.SendMessage(channelID, "Video is too large to embed!")
		return
	}

	if _, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:        fmt.Sprintf("%s.mp4", fileName),
				ContentType: "video/mp4",
				Reader:      bytes.NewReader(buf.Bytes()),
			},
		},
	}); err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send video")
	}
}

func (s *Session) SendVideoURL(channelID string, videoURL string, fileName string) {
	s.sendVideoURL(channelID, videoURL, fileName, s.GetGuildPremiumTier(channelID))
}

func (s *Session) sendVideoURL(channelID string, videoURL string, fileName string, tier discordgo.PremiumTier) {
	if videoURL == "" {
		return
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	file, _, err := utils.Download(videoURL, GetMessageMaxBytes(tier))
	if err != nil {
		// Video is too big, just send the URL
		s.SendMessage(channelID, videoURL)
		return
	}

	if _, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:        fmt.Sprintf("%s.mp4", fileName),
				ContentType: "video/mp4",
				Reader:      file,
			},
		},
	}); err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send video")
	}
}

func (s *Session) SendVideoURLs(channelID string, videoURLs []string, fileNamePrefix string) {
	s.sendVideoURLs(channelID, videoURLs, fileNamePrefix, s.GetGuildPremiumTier(channelID))
}

func (s *Session) sendVideoURLs(channelID string, videoURLs []string, fileNamePrefix string, tier discordgo.PremiumTier) {
	if len(videoURLs) == 0 {
		return
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	maxBytes := GetMessageMaxBytes(tier)

	messages := make([]*discordgo.MessageSend, 0, len(videoURLs))
	files := make([]*discordgo.File, 0, len(videoURLs))

	remainingBytes := maxBytes
	for i, url := range videoURLs {
		file, bytes, err := utils.Download(url, remainingBytes)
		if errors.Is(err, utils.ErrResponseTooLong) && bytes < maxBytes {
			// This video can't fit in the current message, but does not exceed the maximum bytes in a single message
			// Flush the current files to a message and start a new message
			// Then redownload the video
			messages = append(messages, &discordgo.MessageSend{
				Files: files,
			})
			files = make([]*discordgo.File, 0)
			remainingBytes = maxBytes

			file, bytes, err = utils.Download(url, remainingBytes)
		}

		if (err != nil && !errors.Is(err, utils.ErrResponseTooLong)) || bytes > maxBytes {
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

	for _, message := range messages {
		if _, err := s.ChannelMessageSendComplex(channelID, message); err != nil {
			s.Logger.With(zap.Error(err)).Error("Failed to send video")
			continue
		}
	}
}

func (s *Session) SendMP4URLAsGIF(channelID string, videoURL string, fileName string) {
	s.sendMP4URLAsGIF(channelID, videoURL, fileName, s.GetGuildPremiumTier(channelID))
}

func (s *Session) sendMP4URLAsGIF(channelID string, videoURL string, fileName string, tier discordgo.PremiumTier) {
	if videoURL == "" {
		return
	}

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	mp4File, _, err := utils.Download(videoURL, GetMessageMaxBytes(tier))
	if err != nil {
		// Since converting it to a GIF will make the video bigger
		// If the video is already too big, then just send the URL
		s.Logger.With(zap.Error(err)).Error("Failed to download MP4")
		s.SendVideoURL(channelID, videoURL, fileName)
		return
	}

	mp4FileBytes, err := io.ReadAll(mp4File)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to read MP4 before converting to GIF")
		s.SendVideoURL(channelID, videoURL, fileName)
		return
	}

	gifBytes, err := utils.ConvertMP4ToGIF(mp4FileBytes)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to convert GIF")
		s.SendVideoURL(channelID, videoURL, fileName)
		return
	}

	if _, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:        fmt.Sprintf("%s.gif", fileName),
				ContentType: "video/mp4",
				Reader:      bytes.NewReader(gifBytes),
			},
		},
	}); err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send as GIF")
		s.SendVideoURL(channelID, videoURL, fileName)
		return
	}
}

func (s *Session) SendBytesProgressBar(channelID string, totalBytes int64, description ...string) (*ProgressBar, error) {
	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return nil, ErrMissingPermissions
	}

	m, err := s.ChannelMessageSend(channelID, "Creating progress bar...")
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to create progress bar")
		return nil, err
	}

	return NewBytesProgressBar(s, m, totalBytes, description...), nil
}

func (s *Session) SendError(channelID string, errToSend error) {
	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	if _, err := s.ChannelMessageSend(channelID, errToSend.Error()); err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send error message")
	}
}

func (s *Session) SendErrorf(channelID string, format string, a ...any) {
	s.SendError(channelID, fmt.Errorf(format, a...))
}

func (s *Session) SendInternalError(channelID string, errToLog error) {
	s.SendInternalErrorWithMessage(channelID, errToLog, "An internal error has occurred when processing this message")
}

func (s *Session) SendInternalErrorWithMessage(channelID string, errToLog error, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)

	s.Logger.With(zap.Error(errToLog)).Error("Unexpected error")

	hasPermissions, err := s.HasSendMessagePermissions(channelID)
	if err != nil || !hasPermissions {
		return
	}

	if _, err := s.ChannelMessageSend(channelID, msg); err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send error message")
	}
}

type MessageSession struct {
	*Session
	*discordgo.Message
}

func NewMessageSession(s *discordgo.Session, m *discordgo.Message, l *zap.SugaredLogger) *MessageSession {
	return &MessageSession{
		Session: NewSession(s, l),
		Message: m,
	}
}

func (s *MessageSession) GetMessageEmbeds() (UpdatableMessageEmbeds, error) {
	return s.Session.GetMessageEmbeds(s.ChannelID, s.Message.ID)
}

func (s *MessageSession) SendMessage(format string, a ...any) {
	s.Session.SendMessage(s.ChannelID, format, a...)
}

func (s *MessageSession) SendEmbed(embed *discordgo.MessageEmbed) (*UpdatableMessageEmbed, error) {
	return s.Session.SendEmbed(s.ChannelID, embed)
}

func (s *MessageSession) DownloadImageAndSendEmbed(embed *discordgo.MessageEmbed, fileName string) (*UpdatableMessageEmbed, error) {
	return s.Session.downloadImageAndSendEmbed(s.ChannelID, embed, fileName, s.GetGuildPremiumTier())
}

func (s *MessageSession) SendEmbeds(embeds []*discordgo.MessageEmbed) (UpdatableMessageEmbeds, error) {
	return s.Session.SendEmbeds(s.ChannelID, embeds)
}

func (s *MessageSession) SendBytesProgressBar(totalBytes int64, description ...string) (*ProgressBar, error) {
	return s.Session.SendBytesProgressBar(s.ChannelID, totalBytes, description...)
}

func (s *MessageSession) SendVideo(video io.ReadCloser, fileName string) {
	s.Session.sendVideo(s.ChannelID, video, fileName, s.GetGuildPremiumTier())
}

func (s *MessageSession) SendVideoURL(videoURL string, fileName string) {
	s.Session.sendVideoURL(s.ChannelID, videoURL, fileName, s.GetGuildPremiumTier())
}

func (s *MessageSession) SendVideoURLs(videoURLs []string, fileNamePrefix string) {
	s.Session.sendVideoURLs(s.ChannelID, videoURLs, fileNamePrefix, s.GetGuildPremiumTier())
}

func (s *MessageSession) SendMP4URLAsGIF(videoURL string, fileName string) {
	s.Session.sendMP4URLAsGIF(s.ChannelID, videoURL, fileName, s.GetGuildPremiumTier())
}

func (s *MessageSession) SendError(errToSend error) {
	s.Session.SendError(s.ChannelID, errToSend)
}

func (s *MessageSession) SendErrorf(format string, a ...any) {
	s.Session.SendErrorf(s.ChannelID, format, a...)
}

func (s *MessageSession) SendInternalError(errToLog error) {
	s.Session.SendInternalError(s.ChannelID, errToLog)
}

func (s *MessageSession) SendInternalErrorWithMessage(errToLog error, format string, a ...any) {
	s.Session.SendInternalErrorWithMessage(s.ChannelID, errToLog, format, a...)
}

func (s *MessageSession) GetGuildPremiumTier() discordgo.PremiumTier {
	guild, err := s.Guild(s.GuildID)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to get guild")
		return discordgo.PremiumTierNone
	}

	return guild.PremiumTier
}

func processEmbeds(embeds []*discordgo.MessageEmbed) []*discordgo.MessageEmbed {
	ret := make([]*discordgo.MessageEmbed, 0)
	for _, embed := range embeds {
		ret = append(ret, processEmbed(embed))
	}
	return ret
}

func processEmbed(embed *discordgo.MessageEmbed) *discordgo.MessageEmbed {
	embed.Title = trimStringToLength(embed.Title, 256)
	embed.Description = trimStringToLength(embed.Description, 4096)

	if len(embed.Fields) > 25 {
		embed.Fields = embed.Fields[:25]
	}

	for _, field := range embed.Fields {
		field.Name = trimStringToLength(field.Name, 256)
		field.Value = trimStringToLength(field.Value, 1024)
	}

	if embed.Footer != nil {
		embed.Footer.Text = trimStringToLength(embed.Footer.Text, 2048)
	}

	if embed.Author != nil {
		embed.Author.Name = trimStringToLength(embed.Author.Name, 256)
	}

	return embed
}

func trimStringToLength(s string, maxChars int) string {
	runes := []rune(s)

	if len(runes) <= maxChars {
		return s
	}

	runes = runes[:maxChars-1]
	runes = append(runes, '…')
	ret := string(runes)
	return ret
}
