package logging

import (
	"context"
	"log/slog"
	"os"
)

type loggerKey string

const key loggerKey = "logger"

// New returns a JSON logger writing to stderr. JSON (rather than logfmt) so log
// shippers (e.g. Vector) can parse each line into structured fields; the
// message lands in slog's default "msg" key.
func New() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(jsonHandler)
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
