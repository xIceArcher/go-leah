package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/bot"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/handler"
	"github.com/xIceArcher/go-leah/logger"
	"go.uber.org/zap"
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

	bot, err := bot.New(cfg, discordgo.IntentsGuilds|discordgo.IntentsGuildMessages, logger)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to initialize bot")
	}
	defer logger.Info("Bot shut down")

	logger.Info("Starting bot...")
	if err := bot.Start(); err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to start bot")
	}
	defer bot.Stop()
	logger.Info("Bot started")

	logger.Info("Initializing command handler...")
	commandHandler, err := handler.NewCommandHandler(cfg, bot.Session)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to initialize command handler")
	}
	defer commandHandler.Stop()
	logger.Info("Initialized command handler")

	bot.AddHandler(commandHandler)

	logger.Info("Initializing regex handler...")
	regexHandler, err := handler.NewRegexHandler(cfg, bot.Session)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to initialize regex handler")
	}
	defer regexHandler.Stop()
	logger.Info("Initialized regex handler")

	bot.AddHandler(regexHandler)

	logger.Info("Bot running")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
	logger.Info("Shutting down bot...")
}
