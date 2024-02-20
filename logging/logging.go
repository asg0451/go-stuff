package logging

import (
	"context"
	"log/slog"
	"os"
)

type loggerKey string

const key loggerKey = "logger"

func New() *slog.Logger {
	textHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(textHandler)
}

func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, key, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(key).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return l
}
