// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"encoding/hex"
	"strconv"
	"sync/atomic"
	"time"

	"gitea.dev/modules/otelexporter"
)

// OtelTraceConfig is the exporter configuration, set once at startup by
// cmd/web.go from the "otel_exporter" settings. gtprof does not read the
// settings package itself.
type OtelTraceConfig struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string
	Headers        map[string]string
	Timeout        time.Duration
	GzipEnabled    bool
	TLSInsecure    bool
}

var otelTraceConfig atomic.Pointer[OtelTraceConfig]

// EnableOtelTracer enables OTLP/HTTP JSON export of eligible span trees.
// Tracing itself (span collection and the builtin threshold report) works
// with or without this exporter; when it is not enabled, no payload is ever
// sent.
func EnableOtelTracer(cfg *OtelTraceConfig) {
	otelTraceConfig.Store(cfg)
	otelexporter.Init(
		cfg.Endpoint,
		cfg.Headers,
		cfg.Timeout,
		cfg.GzipEnabled,
		cfg.TLSInsecure,
	)
}

// DisableOtelTracer stops the exporter and drains pending traces.
func DisableOtelTracer(timeout time.Duration) {
	if otelTraceConfig.Swap(nil) != nil {
		otelexporter.StopDefault(timeout)
	}
}

func otelExportIfEligible(t *traceBuiltinSpan, slow bool) {
	cfg := otelTraceConfig.Load()
	exporter := otelexporter.Default()
	if cfg == nil || exporter == nil {
		return
	}
	// The default sampling policy mirrors the builtin report: slow operations
	// are always exported; fast ones only when the caller propagated a sampled
	// W3C trace context, so a caller that wants the full trace of one
	// operation gets it without exporting every fast request.
	root := t.ts
	if !slow && !root.sampled {
		return
	}
	exporter.ExportTrace(otelBuildTrace(cfg, root))
}

func otelBuildTrace(cfg *OtelTraceConfig, root *TraceSpan) *otelexporter.OtelTrace {
	scopeSpan := &otelexporter.OtelScopeSpan{
		Scope: &otelexporter.OtelScope{Name: "gitea-server", Version: cfg.ServiceVersion},
	}
	resSpans := &otelexporter.OtelResourceSpan{
		Resource: &otelexporter.OtelResource{
			Attributes: []*otelexporter.OtelAttribute{
				{Key: "service.name", Value: otelexporter.OtelAttributeStringValue{StringValue: cfg.ServiceName}},
				{Key: "service.version", Value: otelexporter.OtelAttributeStringValue{StringValue: cfg.ServiceVersion}},
			},
		},
		ScopeSpans: []*otelexporter.OtelScopeSpan{scopeSpan},
	}
	otelAppendSpan(scopeSpan, root)
	return &otelexporter.OtelTrace{ResourceSpans: []*otelexporter.OtelResourceSpan{resSpans}}
}

// otelAppendSpan converts one TraceSpan (and its children) to an OTLP span.
// The trace/span IDs are the ones assigned at span start: a root span that
// joined a caller's W3C trace keeps that trace ID and parent, so the exported
// tree lands inside the caller's trace.
func otelAppendSpan(scopeSpan *otelexporter.OtelScopeSpan, ts *TraceSpan) {
	ts.mu.RLock()
	span := &otelexporter.OtelSpan{
		TraceID:           hex.EncodeToString(ts.traceID[:]),
		SpanID:            hex.EncodeToString(ts.spanID[:]),
		ParentSpanID:      hex.EncodeToString(ts.parentSpanID[:]),
		Name:              ts.name,
		StartTimeUnixNano: strconv.FormatInt(ts.startTime.UnixNano(), 10),
		EndTimeUnixNano:   strconv.FormatInt(ts.endTime.UnixNano(), 10),
		Status:            int(ts.statusCode),
		StatusMessage:     ts.statusDesc,
	}
	if ts.name == TraceSpanHTTP {
		span.Kind = 2 // OTLP SpanKind SERVER
	} else {
		span.Kind = 1 // OTLP SpanKind INTERNAL
	}
	for _, a := range ts.attributes {
		span.Attributes = append(span.Attributes, otelAttribute(a))
	}
	children := make([]*TraceSpan, len(ts.children))
	copy(children, ts.children)
	ts.mu.RUnlock()

	scopeSpan.Spans = append(scopeSpan.Spans, span)
	for _, child := range children {
		otelAppendSpan(scopeSpan, child)
	}
}

func otelAttribute(a *TraceAttribute) *otelexporter.OtelAttribute {
	switch v := a.Value.v.(type) {
	case string:
		return &otelexporter.OtelAttribute{Key: a.Key, Value: otelexporter.OtelAttributeStringValue{StringValue: v}}
	case bool:
		return &otelexporter.OtelAttribute{Key: a.Key, Value: otelexporter.OtelAttributeBoolValue{BoolValue: v}}
	case float64:
		return &otelexporter.OtelAttribute{Key: a.Key, Value: otelexporter.OtelAttributeDoubleValue{DoubleValue: v}}
	case int, int64:
		return &otelexporter.OtelAttribute{Key: a.Key, Value: otelexporter.OtelAttributeIntValue{IntValue: strconv.FormatInt(a.Value.AsInt64(), 10)}}
	default:
		return &otelexporter.OtelAttribute{Key: a.Key, Value: otelexporter.OtelAttributeStringValue{StringValue: a.Value.AsString()}}
	}
}
