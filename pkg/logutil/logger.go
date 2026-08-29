package logutil

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

const (
	attrMethod = "method"
)

const (
	LevelTrace   = slog.Level(-8)
	LevelDebug   = slog.LevelDebug
	LevelInfo    = slog.LevelInfo
	LevelWarning = slog.LevelWarn
	LevelError   = slog.LevelError
)

const (
	colorBlueIntense      = 12
	colorRedIntense       = 9
	colorLightBlueIntense = 14
	colorIndigoIntense    = 13
	colorGreenIntense     = 10
	colorWhiteIntense     = 15
)

func WithMethod(logger *slog.Logger, method string) *slog.Logger {
	return logger.With(attrMethod, method)
}

func fromLevelString(level string) slog.Level {
	switch level {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarning
	case "error":
		return LevelError
	}
	return LevelInfo
}

func fromFormatString(format string, level slog.Level) *slog.Logger {
	options := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttr,
	}

	switch format {
	case "color":
		return slog.New(tint.NewHandler(os.Stderr, &tint.Options{
			Level:       level,
			TimeFormat:  time.Kitchen,
			ReplaceAttr: replaceAttr,
		}))
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stderr, options))
	case "none":
		fallthrough
	default:
		return slog.New(slog.NewTextHandler(os.Stderr, options))
	}
}

func NewLogger(level string, format string) *slog.Logger {
	return fromFormatString(format, fromLevelString(level))
}

func replaceAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.LevelKey {
		if level, ok := attr.Value.Any().(slog.Level); ok && level < LevelDebug {
			attr.Value = slog.StringValue("TRACE")
		}
	}

	if attr.Key == attrMethod {
		switch attr.Value.String() {
		case http.MethodConnect:
			return attr
		case http.MethodGet:
			return tint.Attr(colorBlueIntense, attr)
		case http.MethodDelete:
			return tint.Attr(colorRedIntense, attr)
		case http.MethodPost:
			return tint.Attr(colorLightBlueIntense, attr)
		case http.MethodPatch:
			return tint.Attr(colorIndigoIntense, attr)
		case http.MethodPut:
			return tint.Attr(colorGreenIntense, attr)
		case http.MethodTrace:
			return tint.Attr(colorWhiteIntense, attr)
		}
	}
	return attr
}

func init() {
	slog.SetDefault(NewLogger("trace", "color"))
}
