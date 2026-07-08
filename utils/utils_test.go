package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.coldcutz.net/go-stuff/logging"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestLoggingInterceptorCorrelation asserts that the request logger the
// interceptor places on the context carries both the inbound x-request-id and
// the active span's trace_id/span_id, so a log line can be pivoted to its trace.
func TestLoggingInterceptorCorrelation(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	tid, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatal(err)
	}
	sid, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-request-id", "req-123"))

	interceptor := MakeLoggingInterceptor(log)
	handler := func(ctx context.Context, _ any) (any, error) {
		// Uses the context logger the interceptor installed.
		logging.FromContext(ctx).Info("inside handler")
		return "ok", nil
	}
	if _, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if !strings.Contains(line, "inside handler") {
			continue
		}
		found = true
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("handler log line not valid json: %v", err)
		}
		if got := m["request_id"]; got != "req-123" {
			t.Errorf("request_id = %v, want req-123", got)
		}
		if got := m["trace_id"]; got != tid.String() {
			t.Errorf("trace_id = %v, want %s", got, tid.String())
		}
		if got := m["span_id"]; got != sid.String() {
			t.Errorf("span_id = %v, want %s", got, sid.String())
		}
	}
	if !found {
		t.Fatalf("did not find handler log line in output:\n%s", buf.String())
	}
}
