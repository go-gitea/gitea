// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// "vendor span" is a simple demo for a span from a vendor library

var vendorContextKey any = "vendorContextKey"

type vendorSpan struct {
	name     string
	children []*vendorSpan
}

func vendorTraceStart(ctx context.Context, name string) (context.Context, *vendorSpan) {
	span := &vendorSpan{name: name}
	parentSpan, ok := ctx.Value(vendorContextKey).(*vendorSpan)
	if ok {
		parentSpan.children = append(parentSpan.children, span)
	}
	ctx = context.WithValue(ctx, vendorContextKey, span)
	return ctx, span
}

// below "testTrace*" integrate the vendor span into our trace system

type testTraceSpan struct {
	vendorSpan *vendorSpan
}

func (t *testTraceSpan) addEvent(name string, cfg *EventConfig) {}

func (t *testTraceSpan) recordError(err error, cfg *EventConfig) {}

func (t *testTraceSpan) end() {}

type testTraceStarter struct{}

func (t *testTraceStarter) start(ctx context.Context, traceSpan *TraceSpan, internalSpanIdx int) (context.Context, traceSpanInternal) {
	ctx, span := vendorTraceStart(ctx, traceSpan.name)
	return ctx, &testTraceSpan{span}
}

func TestTraceStarter(t *testing.T) {
	globalTraceStarters = []traceStarter{&testTraceStarter{}}

	ctx := t.Context()
	ctx, span := GetTracer().Start(ctx, "root")
	defer span.End()

	func(ctx context.Context) {
		ctx, span := GetTracer().Start(ctx, "span1")
		defer span.End()
		func(ctx context.Context) {
			_, span := GetTracer().Start(ctx, "spanA")
			defer span.End()
		}(ctx)
		func(ctx context.Context) {
			_, span := GetTracer().Start(ctx, "spanB")
			defer span.End()
		}(ctx)
	}(ctx)

	func(ctx context.Context) {
		_, span := GetTracer().Start(ctx, "span2")
		defer span.End()
	}(ctx)

	var spanFullNames []string
	var collectSpanNames func(parentFullName string, s *vendorSpan)
	collectSpanNames = func(parentFullName string, s *vendorSpan) {
		fullName := parentFullName + "/" + s.name
		spanFullNames = append(spanFullNames, fullName)
		for _, c := range s.children {
			collectSpanNames(fullName, c)
		}
	}
	collectSpanNames("", span.internalSpans[0].(*testTraceSpan).vendorSpan)
	assert.Equal(t, []string{
		"/root",
		"/root/span1",
		"/root/span1/spanA",
		"/root/span1/spanB",
		"/root/span2",
	}, spanFullNames)
}
