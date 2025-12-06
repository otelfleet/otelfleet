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

func init() {
	w := os.Stderr

	// Create a new logger

	// Set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
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
			},
		}),
	))
}
