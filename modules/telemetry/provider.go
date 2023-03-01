// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package telemetry

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func InitProvider(parentCtx context.Context) (func(context.Context) error, error) {
	ctx, _, finished := process.GetManager().AddTypedContext(parentCtx, "Service: OpenTelemetry Exporter", "", process.SystemProcessType, false)
	pprof.SetGoroutineLabels(ctx)
	defer pprof.SetGoroutineLabels(parentCtx)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceName(setting.Telemetry.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var traceExporter sdktrace.SpanExporter

	switch setting.Telemetry.EndpointType {
	case "jaeger":
		opts := []jaeger.CollectorEndpointOption{}
		if setting.Telemetry.Endpoint != "" {
			opts = append(opts, jaeger.WithEndpoint(setting.Telemetry.Endpoint))
		}
		traceExporter, err = jaeger.New(jaeger.WithCollectorEndpoint(opts...))
		if err != nil {
			defer finished()
			return nil, fmt.Errorf("unable to initialize exporter: %w", err)
		}
	case "file", "stdout", "stderr":
		opts := []stdouttrace.Option{}
		if setting.Telemetry.EndpointType == "file" && setting.Telemetry.Endpoint != "" {
			name := setting.Telemetry.Endpoint
			if !filepath.IsAbs(name) {
				name = filepath.Join(setting.AppDataPath, name)
			}
			file, err := os.Open(name)
			if err != nil {
				return nil, fmt.Errorf("unable to open %s for telemetry data to: %w", name, err)
			}
			opts = append(opts, stdouttrace.WithWriter(file))
			finished = func() {
				finished()
				_ = file.Close()
			}
		} else if setting.Telemetry.EndpointType == "stderr" {
			opts = append(opts, stdouttrace.WithWriter(os.Stderr))
		}
		if setting.Telemetry.PrettyPrint {
			opts = append(opts, stdouttrace.WithPrettyPrint())
		}
		if !setting.Telemetry.Timestamps {
			opts = append(opts, stdouttrace.WithoutTimestamps())
		}

		traceExporter, err = stdouttrace.New(opts...)
		if err != nil {
			defer finished()
			return nil, fmt.Errorf("unable to initialize exporter: %w", err)
		}
	case "http", "https":
		// Set up a trace exporter
		options := []otlptracehttp.Option{}
		if setting.Telemetry.Endpoint != "" {
			options = append(options, otlptracehttp.WithEndpoint(setting.Telemetry.Endpoint))
		}
		if setting.Telemetry.EndpointType == "http" {
			options = append(options, otlptracehttp.WithInsecure())
		}
		if !setting.Telemetry.UseTLS {
			options = append(options, otlptracehttp.WithTLSClientConfig(&tls.Config{
				InsecureSkipVerify: true,
			}))
		}

		traceExporter, err = otlptracehttp.New(ctx, options...)
		if err != nil {
			defer finished()
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}

	case "grpc":
		opts := []otlptracegrpc.Option{}
		if !setting.Telemetry.UseTLS {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if setting.Telemetry.Endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(setting.Telemetry.Endpoint))
		}

		// Set up a trace exporter
		traceExporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			defer finished()
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return func(ctx context.Context) error {
		defer finished()

		ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: OpenTelemetry Exporter", "", process.SystemProcessType, true)
		defer finished()

		return tracerProvider.Shutdown(ctx)
	}, nil
}
