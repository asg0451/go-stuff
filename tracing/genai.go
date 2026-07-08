package tracing

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetry GenAI semantic-convention attribute keys. Defined here rather
// than pulled from a semconv package so every service annotates LLM spans with
// the same keys, independent of semconv-package version drift.
const (
	GenAIOperationName  = attribute.Key("gen_ai.operation.name")
	GenAISystem         = attribute.Key("gen_ai.system")
	GenAIRequestModel   = attribute.Key("gen_ai.request.model")
	GenAIRequestTemp    = attribute.Key("gen_ai.request.temperature")
	GenAIRequestMaxTok  = attribute.Key("gen_ai.request.max_tokens")
	GenAIRequestFreqPen = attribute.Key("gen_ai.request.frequency_penalty")
	GenAIUsageInputTok  = attribute.Key("gen_ai.usage.input_tokens")
	GenAIUsageOutputTok = attribute.Key("gen_ai.usage.output_tokens")
	GenAIResponseFinish = attribute.Key("gen_ai.response.finish_reasons")
	GenAIUsageCostUSD   = attribute.Key("gen_ai.usage.cost_usd")
)

// LLMRequest is the request side of an LLM call, for span annotation.
type LLMRequest struct {
	System           string // provider, e.g. "openai" or "x_ai"
	Model            string
	Temperature      float64
	MaxTokens        int
	FrequencyPenalty float64
}

// SetLLMRequest stamps GenAI request attributes (operation "chat") on span.
func SetLLMRequest(span trace.Span, r LLMRequest) {
	span.SetAttributes(
		GenAIOperationName.String("chat"),
		GenAISystem.String(r.System),
		GenAIRequestModel.String(r.Model),
		GenAIRequestTemp.Float64(r.Temperature),
		GenAIRequestMaxTok.Int(r.MaxTokens),
		GenAIRequestFreqPen.Float64(r.FrequencyPenalty),
	)
}

// SetLLMUsage stamps GenAI token-usage attributes on span. Cost is set
// separately (GenAIUsageCostUSD) by callers that can compute it, since not every
// provider exposes pricing.
func SetLLMUsage(span trace.Span, inputTokens, outputTokens int) {
	span.SetAttributes(
		GenAIUsageInputTok.Int(inputTokens),
		GenAIUsageOutputTok.Int(outputTokens),
	)
}
