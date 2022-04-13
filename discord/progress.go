package discord

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/schollz/progressbar/v3"
	"go.uber.org/zap"
)

type ProgressBar struct {
	msg *UpdatableMessage
	raw *progressbar.ProgressBar
}

func NewBytesProgressBar(s *Session, m *discordgo.Message, totalBytes int64, description ...string) *ProgressBar {
	desc := ""
	if len(description) > 0 {
		desc = description[0]
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &ProgressBar{
		msg: NewUpdatableMessage(s, m),
		raw: progressbar.NewOptions64(totalBytes,
			progressbar.OptionSetDescription(desc),
			progressbar.OptionSetWriter(ioutil.Discard),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(10),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        ":white_large_square:",
				SaucerPadding: ":black_large_square:",
				BarStart:      "|",
				BarEnd:        "|",
			}),
			progressbar.OptionOnCompletion(cancel),
		),
	}

	go p.updateTask(ctx)
	return p
}

func (p *ProgressBar) Add(i int64) {
	p.raw.Add64(i)
}

func (p *ProgressBar) Set(i int64) {
	p.raw.Set64(i)
}

func (p *ProgressBar) AddMax(i int64) {
	p.raw.ChangeMax64(p.raw.GetMax64() + i)
}

func (p *ProgressBar) SetMax(i int64) {
	p.raw.ChangeMax64(i)
}

func (p *ProgressBar) updateTask(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			p.msg.Content = p.raw.String()
			if err := p.msg.Update(); err != nil {
				p.msg.Logger.With(zap.Error(err)).Warn("Failed to update progress bar")
			}

			return
		case <-ticker.C:
			currState := p.raw.String()
			if strings.TrimSpace(currState) == "" || p.msg.Content == currState {
				continue
			}

			p.msg.Content = currState
			if err := p.msg.Update(); err != nil {
				p.msg.Logger.With(zap.Error(err)).Warn("Failed to update progress bar")
			}
		}
	}
}
