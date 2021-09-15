package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/xIceArcher/go-leah/bot"
	"github.com/xIceArcher/go-leah/command"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/handler"
	"github.com/xIceArcher/go-leah/logger"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./config.yaml", "Path of configuration file")
	flag.Parse()

	cfg := &config.Config{}
	if err := cfg.LoadConfig(configPath); err != nil {
		log.Fatal(err)
	}

	if err := logger.Init(cfg.Logger); err != nil {
		log.Fatal(err)
	}
	logger := zap.S()
	defer logger.Sync()

	commands, err := command.GetCommands(cfg.Discord.Commands)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to load commands")
	}

	messageHandlers, err := handler.GetHandlers(cfg.Discord.Handlers)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to load handlers")
	}

	bot, err := bot.New(cfg, commands, messageHandlers, discordgo.IntentsGuilds|discordgo.IntentsGuildMessages)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to initialize bot")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	if err := bot.Run(ctx); err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to start bot")
	}
	defer func() {
		bot.Close()
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
}
