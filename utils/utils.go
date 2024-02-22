package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.coldcutz.net/go-stuff/logging"
	"google.golang.org/grpc"
)

func DotEnv() error {
	cnt, err := os.ReadFile(".env")
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read .env: %w", err)
		}
		return nil
	}
	for _, line := range strings.Split(string(cnt), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		os.Setenv(parts[0], parts[1])
	}
	return nil
}

func MakeLoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log.Info("handling request", "method", info.FullMethod, "request", req)
		start := time.Now()
		resp, err := handler(ctx, req)
		if err != nil {
			log.Warn("request failed", "method", info.FullMethod, "request", req, "error", err)
		} else {
			dur := time.Since(start)
			log.Info("request succeeded", "method", info.FullMethod, "request", req, "response", resp, "duration", dur)
		}
		return resp, err
	}
}

// StdSetup is a helper function to setup a logger and context that cancels on ctrl-c or sigterm and has the logger in it
func StdSetup() (ctx context.Context, done context.CancelFunc, log *slog.Logger, err error) {
	if err := DotEnv(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read .env: %w", err)
	}
	log = logging.New()
	slog.SetDefault(log)
	slog.SetLogLoggerLevel(slog.LevelInfo)
	ctx = logging.NewContext(context.Background(), log)
	ctx, done = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer done()

	return ctx, done, log, nil
}
