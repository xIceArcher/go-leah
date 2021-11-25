package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/xIceArcher/go-leah/bot"
	"github.com/xIceArcher/go-leah/cog"
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

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cogs, err := cog.SetupCogs(ctx, cfg, &wg, logger)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to load cogs")
	}

	handlers, err := handler.SetupHandlers(ctx, cfg, &wg, logger)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to load handlers")
	}

	bot, err := bot.New(cfg, cogs, handlers, discordgo.IntentsGuilds|discordgo.IntentsGuildMessages)
	if err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to initialize bot")
	}

	if err := bot.Run(ctx); err != nil {
		logger.With(zap.Error(err)).Fatal("Failed to start bot")
	}
	defer bot.Close()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
}
