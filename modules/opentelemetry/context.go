// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package opentelemetry

import (
	"context"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

type Context struct {
	context.Context
	tp *trace.TracerProvider
	mp *metric.MeterProvider
}

var DefaultContext *Context

func GetTracerProvider(ctx context.Context) *trace.TracerProvider {
	if telemetryCtx, ok := ctx.(*Context); ok {
		return telemetryCtx.tp
	}
	return DefaultContext.tp
}

func GetMeterProvider(ctx context.Context) *metric.MeterProvider {
	if telemetryCtx, ok := ctx.(*Context); ok {
		return telemetryCtx.mp
	}
	return DefaultContext.mp
}

func SetDefaultProviders(ctx context.Context, tp *trace.TracerProvider, mp *metric.MeterProvider) {
	DefaultContext = &Context{
		Context: ctx,
		tp:      tp,
		mp:      mp,
	}
}
