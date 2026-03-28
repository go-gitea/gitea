// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build otel

package gtprof

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

// OTELConfig holds the configuration for OpenTelemetry
type OTELConfig struct {
	Enabled        bool
	ServiceName    string
	TracesEndpoint string
	Headers        map[string]string
	Insecure       bool
	Compression    string
	Timeout        time.Duration
}

type traceOtelStarter struct {
	config         *OTELConfig
	tracerProvider *sdktrace.TracerProvider
	initialized    bool
}

type traceOtelSpan struct {
	span      trace.Span
	traceSpan *TraceSpan
}

// noopOtelSpan is a no-op implementation when OTEL is not enabled
type noopOtelSpan struct{}

func (n *noopOtelSpan) addEvent(name string, cfg *EventConfig)  {}
func (n *noopOtelSpan) recordError(err error, cfg *EventConfig) {}
func (n *noopOtelSpan) end()                                    {}

func (t *traceOtelSpan) addEvent(name string, cfg *EventConfig) {
	if cfg == nil {
		t.span.AddEvent(name)
		return
	}

	attrs := make([]attribute.KeyValue, 0, len(cfg.attributes))
	for _, attr := range cfg.attributes {
		attrs = append(attrs, convertAttribute(attr))
	}

	opts := []trace.EventOption{
		trace.WithAttributes(attrs...),
	}

	t.span.AddEvent(name, opts...)
}

func (t *traceOtelSpan) recordError(err error, cfg *EventConfig) {
	opts := []trace.EventOption{}

	if cfg != nil && len(cfg.attributes) > 0 {
		attrs := make([]attribute.KeyValue, 0, len(cfg.attributes))
		for _, attr := range cfg.attributes {
			attrs = append(attrs, convertAttribute(attr))
		}
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	t.span.RecordError(err, opts...)
	t.span.SetStatus(codes.Error, err.Error())
}

func (t *traceOtelSpan) end() {
	// Sync all final attributes before ending the span
	t.traceSpan.mu.RLock()
	if len(t.traceSpan.attributes) > 0 {
		attrs := make([]attribute.KeyValue, 0, len(t.traceSpan.attributes))
		for _, attr := range t.traceSpan.attributes {
			attrs = append(attrs, convertAttribute(attr))
		}
		t.span.SetAttributes(attrs...)
	}
	t.traceSpan.mu.RUnlock()

	t.span.End()
}

func (t *traceOtelStarter) start(ctx context.Context, traceSpan *TraceSpan, internalSpanIdx int) (context.Context, traceSpanInternal) {
	if !t.initialized || t.config == nil || !t.config.Enabled {
		return ctx, &noopOtelSpan{}
	}

	tracer := otel.Tracer("code.gitea.io/gitea")

	opts := []trace.SpanStartOption{
		trace.WithTimestamp(traceSpan.startTime),
	}

	// Add initial attributes if any
	traceSpan.mu.RLock()
	if len(traceSpan.attributes) > 0 {
		attrs := make([]attribute.KeyValue, 0, len(traceSpan.attributes))
		for _, attr := range traceSpan.attributes {
			attrs = append(attrs, convertAttribute(attr))
		}
		opts = append(opts, trace.WithAttributes(attrs...))
	}
	spanName := traceSpan.name
	traceSpan.mu.RUnlock()

	newCtx, span := tracer.Start(ctx, spanName, opts...)

	return newCtx, &traceOtelSpan{span: span, traceSpan: traceSpan}
}

func convertAttribute(attr *TraceAttribute) attribute.KeyValue {
	switch v := attr.Value.v.(type) {
	case string:
		return attribute.String(attr.Key, v)
	case int:
		return attribute.Int(attr.Key, v)
	case int64:
		return attribute.Int64(attr.Key, v)
	case float64:
		return attribute.Float64(attr.Key, v)
	case bool:
		return attribute.Bool(attr.Key, v)
	default:
		return attribute.String(attr.Key, attr.Value.AsString())
	}
}

var otelTraceStarter = &traceOtelStarter{}

// initialize sets up the OTEL tracer with the given configuration
func (t *traceOtelStarter) initialize(config *OTELConfig) error {
	if t.initialized {
		return nil
	}

	t.config = config

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion("gitea"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	traceExp, err := t.createOTLPTraceExporter(config)
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	t.tracerProvider = tp

	t.initialized = true
	return nil
}

// shutdown gracefully shuts down the OpenTelemetry providers
func (t *traceOtelStarter) shutdown(ctx context.Context) error {
	var err error

	// Shutdown tracer provider
	if t.tracerProvider != nil {
		if tErr := t.tracerProvider.Shutdown(ctx); tErr != nil && err == nil {
			err = tErr
		}
	}

	return err
}

func (t *traceOtelStarter) createOTLPTraceExporter(config *OTELConfig) (sdktrace.SpanExporter, error) {
	tracesURL := config.TracesEndpoint
	if !strings.Contains(tracesURL, "://") {
		if config.Insecure {
			tracesURL = "http://" + tracesURL
		} else {
			tracesURL = "https://" + tracesURL
		}
	}
	if !strings.HasSuffix(tracesURL, "/v1/traces") {
		tracesURL += "/v1/traces"
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(tracesURL),
		otlptracehttp.WithTimeout(config.Timeout),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     5 * time.Second,
			MaxElapsedTime:  30 * time.Second,
		}),
	}

	if len(config.Headers) > 0 {
		headers := make(map[string]string)
		for k, v := range config.Headers {
			headers[k] = v
		}
		opts = append(opts, otlptracehttp.WithHeaders(headers))
	}

	if config.Compression == "gzip" {
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
	}

	if config.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	client := otlptracehttp.NewClient(opts...)
	return otlptrace.New(context.Background(), client)
}

// EnableOTELTracer enables OpenTelemetry tracing with the given configuration
func EnableOTELTracer(config *OTELConfig) error {
	if config == nil || !config.Enabled {
		return nil
	}

	return otelTraceStarter.initialize(config)
}

// ShutdownOTELTracer gracefully shuts down the OpenTelemetry tracer
func ShutdownOTELTracer(ctx context.Context) error {
	return otelTraceStarter.shutdown(ctx)
}

func init() {
	// Register the OTEL tracer starter - it will be active only if OTEL is enabled
	globalTraceStarters = append(globalTraceStarters, otelTraceStarter)
}
