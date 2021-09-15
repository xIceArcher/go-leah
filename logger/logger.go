package logger

import (
	"go.uber.org/zap"
)

func Init(cfg *zap.Config) error {
	cfg.EncoderConfig = zap.NewDevelopmentEncoderConfig()

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}
