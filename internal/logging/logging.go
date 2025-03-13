package logging

import (
	"io"
	"strings"

	"go.lepovirta.org/otk/internal/envvar"
	"github.com/rs/zerolog"
)

type Config struct {
	Format     string     `json:"format"`
	Level      string     `json:"level"`
	FieldNames FieldNames `json:"fieldNames"`
	TimeFormat string     `json:"timeFormat"`
}

type FieldNames struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level"`
}

func (this *Config) FromEnv(appName string, envVars envvar.Vars) {
	this.Level = envVars.GetForApp(appName, "LOG_LEVEL")
	this.Format = envVars.GetForApp(appName, "LOG_FORMAT")
	this.TimeFormat = envVars.GetForApp(appName, "LOG_TIME_FORMAT")
	this.FieldNames.FromEnv(appName, envVars)
}

func (this *FieldNames) FromEnv(appName string, envVars envvar.Vars) {
	this.Timestamp = envVars.GetForAppOr(
		appName, "LOG_FIELD_TIMESTAMP", zerolog.TimestampFieldName,
	)
	this.Message = envVars.GetForAppOr(
		appName, "LOG_FIELD_MESSAGE", zerolog.MessageFieldName,
	)
	this.Level = envVars.GetForAppOr(
		appName, "LOG_FIELD_LEVEL", zerolog.LevelFieldName,
	)
}

func (this *Config) SetupGlobal(
	appName string,
	outStream io.Writer,
) {
	var err error

	// Logging fields
	zerolog.TimeFieldFormat = this.TimeFormat
	zerolog.TimestampFieldName = this.FieldNames.Timestamp
	zerolog.MessageFieldName = this.FieldNames.Message
	zerolog.LevelFieldName = this.FieldNames.Level

	logger := zerolog.New(outStream).
		With().
		Timestamp().
		Str("app", appName).
		Logger()

	// Logging level
	var level zerolog.Level
	if this.Level == "" {
		level = zerolog.InfoLevel
	} else {
		level, err = zerolog.ParseLevel(this.Level)
		if err != nil {
			level = zerolog.InfoLevel
		}
	}
	logger = logger.Level(level)
	zerolog.SetGlobalLevel(level)

	// Log format
	output := logFormatToOutput(this.Format, outStream)
	if output != nil {
		logger = logger.Output(output)
	}

	zerolog.DefaultContextLogger = &logger

	// Log warnings
	if err != nil {
		logger.Warn().Msgf("unknown log level %s", this.Level)
	}
	if output == nil {
		logger.Warn().Msgf("unknown log format: %s", this.Format)
	}
	logger.Debug().Msg("global logging setup done")
}

func baseLogger(appName string, outStream io.Writer) zerolog.Logger {
	return zerolog.New(outStream).
		With().
		Timestamp().
		Str("app", appName).
		Logger()
}

func logFormatToOutput(logFormat string, outStream io.Writer) io.Writer {
	switch strings.ToLower(logFormat) {
	case "json", "":
		return outStream
	case "pretty":
		return zerolog.ConsoleWriter{Out: outStream}
	default:
		return nil
	}
}
