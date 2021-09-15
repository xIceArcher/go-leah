package command

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/config"
)

type DiscordBotCommandHandlerFunc func(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) error

func (handler DiscordBotCommandHandlerFunc) GetCommandName() string {
	fullName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	tokenizedName := strings.Split(fullName, ".")
	return strings.ToLower(tokenizedName[len(tokenizedName)-1])
}

var implementedCommands []DiscordBotCommandHandlerFunc = []DiscordBotCommandHandlerFunc{
	Admin,
	Restart,
	Embed,
	Photos,
	Video,
	Quoted,
}

type DiscordBotCommand struct {
	Name        string
	HandlerFunc DiscordBotCommandHandlerFunc
	Configs     *config.DiscordCommandConfig
}

func GetCommands(toLoad map[string]*config.DiscordCommandConfig) (commands []*DiscordBotCommand, err error) {
	availableFuncs := make(map[string]DiscordBotCommandHandlerFunc)
	for _, handlerFunc := range implementedCommands {
		availableFuncs[handlerFunc.GetCommandName()] = handlerFunc
	}

	for name, cfg := range toLoad {
		handlerFunc, ok := availableFuncs[name]
		if !ok {
			return nil, fmt.Errorf("command %s not found", name)
		}

		if cfg == nil {
			cfg = &config.DiscordCommandConfig{}
		}

		commands = append(commands, &DiscordBotCommand{
			Name:        name,
			HandlerFunc: handlerFunc,
			Configs:     cfg,
		})
	}

	return
}

// Restart is a dummy command that doesn't actually do anything
// This is because restart commands are caught earlier in the call stack
func Restart(ctx context.Context, cfg *config.Config, session *discordgo.Session, msg *discordgo.Message, args []string) error {
	return nil
}
