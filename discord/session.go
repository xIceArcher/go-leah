package discord

import (
	"errors"
	"fmt"

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

	m, err := s.ChannelMessageSendEmbed(channelID, embed)
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
		Embeds: []*discordgo.MessageEmbed{embed},
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

	m, err := s.ChannelMessageSendEmbeds(channelID, embeds)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("Failed to send embeds")
		return nil, err
	}

	return NewUpdatableMessageEmbeds(s, m), nil
}

func (s *Session) SendVideo(channelID string, videoURL string, fileName string) {
	s.sendVideo(channelID, videoURL, fileName, s.GetGuildPremiumTier(channelID))
}

func (s *Session) sendVideo(channelID string, videoURL string, fileName string, tier discordgo.PremiumTier) {
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

func (s *Session) SendVideos(channelID string, videoURLs []string, fileNamePrefix string) {
	s.sendVideos(channelID, videoURLs, fileNamePrefix, s.GetGuildPremiumTier(channelID))
}

func (s *Session) sendVideos(channelID string, videoURLs []string, fileNamePrefix string, tier discordgo.PremiumTier) {
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

func (s *MessageSession) SendVideo(videoURL string, fileName string) {
	s.Session.sendVideo(s.ChannelID, videoURL, fileName, s.GetGuildPremiumTier())
}

func (s *MessageSession) SendVideos(videoURLs []string, fileNamePrefix string) {
	s.Session.sendVideos(s.ChannelID, videoURLs, fileNamePrefix, s.GetGuildPremiumTier())
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
