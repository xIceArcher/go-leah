package logger

import (
	"os"
	"path"

	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Init(cfg *config.LogConfig) error {
	infoWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path.Join(cfg.LogPath, "info.log"),
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     7,
	})

	errorWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path.Join(cfg.LogPath, "error.log"),
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     28,
	})

	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), infoWriter, zap.InfoLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), errorWriter, zap.ErrorLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), os.Stdout, zap.InfoLevel),
	)

	logger := zap.New(core, zap.ErrorOutput(os.Stderr))
	zap.ReplaceGlobals(logger)
	return nil
}
