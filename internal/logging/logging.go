package logging

import (
	"context"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Logging struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

func (this *Logging) SetupContext(
	ctx context.Context,
) context.Context {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.TimestampFieldName = "ts"
	zerolog.MessageFieldName = "message"
	zerolog.LevelFieldName = "level"

	if config.Level == "" {
		config.Level = os.Getenv("GIT_SYNC_LOG_LEVEL")
	}
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		log.Warn().Msgf("unknown log level %s", config.Level)
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if config.Format == "" {
		config.Format = os.Getenv("GIT_SYNC_LOG_FORMAT")
	}
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(level)
	switch strings.ToLower(config.Format) {
	case "json", "":
		break
	case "pretty":
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	default:
		log.Warn().Msgf("unknown log format: %s", config.Format)
	}

	zerolog.DefaultContextLogger = &logger
	return logger.WithContext(ctx)
}
