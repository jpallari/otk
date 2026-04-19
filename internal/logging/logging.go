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

func (c *Config) FromEnv(appName string, envVars envvar.Vars) {
	c.Level = envVars.GetForApp(appName, "LOG_LEVEL")
	c.Format = envVars.GetForApp(appName, "LOG_FORMAT")
	c.FieldNames.FromEnv(appName, envVars)
}

func (fn *FieldNames) FromEnv(appName string, envVars envvar.Vars) {
	fn.Timestamp = envVars.GetForAppOr(
		appName, "LOG_FIELD_TIMESTAMP", slog.TimeKey,
	)
	fn.Message = envVars.GetForAppOr(
		appName, "LOG_FIELD_MESSAGE", slog.MessageKey,
	)
	fn.Level = envVars.GetForAppOr(
		appName, "LOG_FIELD_LEVEL", slog.LevelKey,
	)
	fn.Source = envVars.GetForAppOr(
		appName, "LOG_FIELD_SOURCE", slog.SourceKey,
	)
}

func (fn *FieldNames) mapLogKey(key string) string {
	switch key {
	case slog.TimeKey:
		return fn.Timestamp
	case slog.MessageKey:
		return fn.Message
	case slog.LevelKey:
		return fn.Level
	case slog.SourceKey:
		return fn.Source
	}
	return key
}

func (c *Config) SetupGlobal(
	appName string,
	outStream io.Writer,
) {
	var err error

	var level slog.Level
	switch strings.ToLower(c.Level) {
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
		err = fmt.Errorf("unknown log level %s", c.Level)
	}

	handlerOpts := slog.HandlerOptions{
		AddSource: c.AddSource,
		Level:     level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 {
				a.Key = c.FieldNames.mapLogKey(a.Key)
			}
			return a
		},
	}

	handler := logFormatToOutput(c.Format, outStream, &handlerOpts)
	logger := slog.New(
		handler.WithAttrs([]slog.Attr{
			slog.String("app", appName),
		}),
	)
	slog.SetDefault(logger)

	if err != nil {
		logger.Warn("unknown log level", slog.String("level", c.Level))
	}
	if strings.ToLower(c.Format) != "json" && strings.ToLower(c.Format) != "pretty" && c.Format != "" {
		logger.Warn("unknown log format", slog.String("format", c.Format))
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
