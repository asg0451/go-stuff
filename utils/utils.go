package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"
	"go.coldcutz.net/go-stuff/logging"
	"go.coldcutz.net/go-stuff/tracing"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

// MakeLoggingInterceptor returns a unary server interceptor that logs each
// request and installs a per-request logger on the context (retrieve it with
// logging.FromContext). The request logger carries the inbound x-request-id
// metadata value and, when tracing is enabled, the active span's
// trace_id/span_id so a log line can be pivoted to its trace.
func MakeLoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		reqLog := log
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-request-id"); len(vals) > 0 {
				reqLog = reqLog.With("request_id", vals[0])
			}
		}
		// Correlate logs with the trace started by otelgrpc's server handler (if
		// tracing is enabled), so a log line can be pivoted to its trace.
		if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
			reqLog = reqLog.With("trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
		}
		ctx = logging.NewContext(ctx, reqLog)
		reqLog.Info("handling request", "method", info.FullMethod, "request", req)
		start := time.Now()
		resp, err := handler(ctx, req)
		if err != nil {
			reqLog.Warn("request failed", "method", info.FullMethod, "request", req, "error", err)
		} else {
			dur := time.Since(start)
			reqLog.Info("request succeeded", "method", info.FullMethod, "request", req, "response", resp, "duration", dur)
		}
		return resp, err
	}
}

// StdSetup is a helper function to set up the standard binary environment:
// .env loading, flag parsing into Opts, a JSON logger (set as the slog default
// and stored in the returned context), a context that cancels on ctrl-c or
// SIGTERM, and OpenTelemetry tracing (see the tracing package; a no-op unless
// OTEL_EXPORTER_OTLP_ENDPOINT is set). The trace service name comes from
// OTEL_SERVICE_NAME, falling back to the binary name.
//
// done flushes pending spans (bounded by a short timeout) and should be
// deferred by the caller.
func StdSetup[Opts any]() (ctx context.Context, log *slog.Logger, opts Opts, done func(), err error) {
	if err := DotEnv(); err != nil {
		return nil, nil, opts, nil, fmt.Errorf("failed to read .env: %w", err)
	}

	if _, err := flags.Parse(&opts); err != nil {
		return nil, nil, opts, nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	log = logging.New()
	slog.SetDefault(log)
	slog.SetLogLoggerLevel(slog.LevelInfo)
	ctx = logging.NewContext(context.Background(), log)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	// unregister the signal handler after the first fire, so a second ctrl-c will interrupt as usual
	go func() {
		<-ctx.Done()
		cancel()
	}()

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = filepath.Base(os.Args[0])
	}
	shutdownTracing, err := tracing.InitTracing(ctx, serviceName)
	if err != nil {
		return nil, nil, opts, nil, fmt.Errorf("failed to init tracing: %w", err)
	}
	done = func() {
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		if err := shutdownTracing(sctx); err != nil {
			log.Warn("tracing shutdown failed", "err", err)
		}
	}

	return ctx, log, opts, done, nil
}
