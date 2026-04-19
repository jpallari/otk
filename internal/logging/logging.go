package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"go.lepovirta.org/otk/internal/envvar"
)

type Config struct {
	Format     string     `json:"format"`
	Level      string     `json:"level"`
	FieldNames FieldNames `json:"fieldNames"`
	AddSource  bool       `json:"addSource"`
}

type FieldNames struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level"`
	Source    string `json:"source"`
}

type loggerCtxKey struct{}

func (this *Config) FromEnv(appName string, envVars envvar.Vars) {
	this.Level = envVars.GetForApp(appName, "LOG_LEVEL")
	this.Format = envVars.GetForApp(appName, "LOG_FORMAT")
	this.FieldNames.FromEnv(appName, envVars)
}

func (this *FieldNames) FromEnv(appName string, envVars envvar.Vars) {
	this.Timestamp = envVars.GetForAppOr(
		appName, "LOG_FIELD_TIMESTAMP", slog.TimeKey,
	)
	this.Message = envVars.GetForAppOr(
		appName, "LOG_FIELD_MESSAGE", slog.MessageKey,
	)
	this.Level = envVars.GetForAppOr(
		appName, "LOG_FIELD_LEVEL", slog.LevelKey,
	)
	this.Source = envVars.GetForAppOr(
		appName, "LOG_FIELD_SOURCE", slog.SourceKey,
	)
}

func (this *FieldNames) mapLogKey(key string) string {
	switch key {
	case slog.TimeKey:
		return this.Timestamp
	case slog.MessageKey:
		return this.Message
	case slog.LevelKey:
		return this.Level
	case slog.SourceKey:
		return this.Source
	}
	return key
}

func (this *Config) SetupGlobal(
	appName string,
	outStream io.Writer,
) {
	var err error

	var level slog.Level
	switch strings.ToLower(this.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info", "":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
		err = fmt.Errorf("unknown log level %s", this.Level)
	}

	handlerOpts := slog.HandlerOptions{
		AddSource: this.AddSource,
		Level:     level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 {
				a.Key = this.FieldNames.mapLogKey(a.Key)
			}
			return a
		},
	}

	handler := logFormatToOutput(this.Format, outStream, &handlerOpts)
	logger := slog.New(
		handler.WithAttrs([]slog.Attr{
			slog.String("app", appName),
		}),
	)
	slog.SetDefault(logger)

	if err != nil {
		logger.Warn("unknown log level", slog.String("level", this.Level))
	}
	if strings.ToLower(this.Format) != "json" && strings.ToLower(this.Format) != "pretty" && this.Format != "" {
		logger.Warn("unknown log format", slog.String("format", this.Format))
	}
}

func AddToContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger)
	if ok {
		return logger
	}
	return slog.Default()
}

func logFormatToOutput(
	logFormat string,
	outStream io.Writer,
	opts *slog.HandlerOptions,
) slog.Handler {
	switch strings.ToLower(logFormat) {
	case "json", "":
		return slog.NewJSONHandler(outStream, opts)
	case "pretty":
		return slog.NewTextHandler(outStream, opts)
	default:
		return slog.NewJSONHandler(outStream, opts)
	}
}
