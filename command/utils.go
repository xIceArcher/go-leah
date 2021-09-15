package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
)

func Admin(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) error {
	guilds := make([]string, 0)
	for _, guild := range session.State.Guilds {
		guilds = append(guilds, guild.Name)
	}

	_, err := session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Active servers: %s", strings.Join(guilds, ", ")))
	return err
}
