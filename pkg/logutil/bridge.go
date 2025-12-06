package logutil

import (
	"context"
	"fmt"
	"log/slog"
)

type slogWrapper struct {
	*slog.Logger
}

func (s *slogWrapper) Debugf(_ context.Context, format string, args ...any) {
	s.Logger.Debug(fmt.Sprintf(format, args...))
}

func (s *slogWrapper) Errorf(_ context.Context, format string, args ...any) {
	s.Logger.Error(fmt.Sprintf(format, args...))
}

func NewOpAMPLogger(logger *slog.Logger) *slogWrapper {
	return &slogWrapper{logger}
}
