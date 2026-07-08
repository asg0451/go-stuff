# Go Stuff
![go](https://github.com/asg0451/go-stuff/actions/workflows/go.yml/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/go.coldcutz.net/go-stuff.svg)](https://pkg.go.dev/go.coldcutz.net/go-stuff)

Common setup stuff for how I write go binaries.

## Packages

- **`utils`** — `StdSetup` (the standard binary bootstrap: .env, flags, JSON logger, signal-cancelled context, tracing) and `MakeLoggingInterceptor` (gRPC request logging with request-id and trace correlation).
- **`logging`** — JSON `slog` logger to stderr (stdout stays reserved for program output), plus `NewContext`/`FromContext` for carrying a request-scoped logger.
- **`tracing`** — OpenTelemetry setup (`InitTracing`), span-ending helper (`End`), and GenAI semantic-convention helpers for LLM call spans (`SetLLMRequest`, `SetLLMUsage`).

## Usage

```go
func main() {
	ctx, log, opts, done, err := utils.StdSetup[Options]()
	if err != nil {
		panic(err)
	}
	defer done() // flushes pending spans

	// For gRPC servers: otelgrpc starts/continues a server span per RPC
	// (reading the inbound traceparent); the logging interceptor runs inside
	// that span and correlates log lines with it.
	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(utils.MakeLoggingInterceptor(log)),
	)
	// ...
}
```

Tracing is opt-in: it activates only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (spans export via OTLP/gRPC, e.g. to an in-cluster otel-collector). The trace service name comes from `OTEL_SERVICE_NAME`, falling back to the binary name. W3C trace-context propagation is enabled so traces stitch together across service boundaries.

Inside handlers, use `logging.FromContext(ctx)` to get the request-scoped logger (it carries `request_id`, `trace_id`, `span_id`), and create child spans with:

```go
func (s *Server) DoThing(ctx context.Context) (err error) {
	ctx, span := otel.Tracer("myservice").Start(ctx, "server.DoThing")
	defer func() { tracing.End(span, err) }()
	// ...
}
```

For LLM calls, stamp the GenAI attributes on the span (`tracing.SetLLMRequest`, `tracing.SetLLMUsage`) and record full prompt/completion content as span events — token counts alone are not enough to debug a bad generation.

Other instrumentation that composes with this: [`otelsql`](https://github.com/XSAM/otelsql) for per-query DB spans, [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) for HTTP handlers/clients.
