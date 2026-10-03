// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	rootSpan, ok := span.internalSpans[0].(*testTraceSpan)
	require.True(t, ok)
	collectSpanNames("", rootSpan.vendorSpan)
	assert.Equal(t, []string{
		"/root",
		"/root/span1",
		"/root/span1/spanA",
		"/root/span1/spanB",
		"/root/span2",
	}, spanFullNames)
}

func TestSpanIdentity(t *testing.T) {
	globalTraceStarters = []traceStarter{&traceBuiltinStarter{}}
	defer func() { globalTraceStarters = nil }()

	ctx, root := GetTracer().Start(t.Context(), "root")
	assert.NotZero(t, root.traceID)
	assert.NotZero(t, root.spanID)
	assert.Zero(t, root.parentSpanID)
	assert.False(t, root.sampled)

	_, child := GetTracer().Start(ctx, "child")
	assert.Equal(t, root.traceID, child.traceID)
	assert.Equal(t, root.spanID, child.parentSpanID)
	assert.NotEqual(t, root.spanID, child.spanID)

	// a sibling gets a distinct span ID in the same trace
	_, sibling := GetTracer().Start(ctx, "sibling")
	assert.Equal(t, root.traceID, sibling.traceID)
	assert.NotEqual(t, child.spanID, sibling.spanID)

	// two independent roots get distinct trace IDs
	_, root2 := GetTracer().Start(t.Context(), "root2")
	assert.NotEqual(t, root.traceID, root2.traceID)
}

func TestParseTraceparent(t *testing.T) {
	tc, ok := ParseTraceparent("00-11111111111111111111111111111111-2222222222222222-01")
	require.True(t, ok)
	assert.Equal(t, "11111111111111111111111111111111", hex.EncodeToString(tc.TraceID[:]))
	assert.Equal(t, "2222222222222222", hex.EncodeToString(tc.SpanID[:]))
	assert.True(t, tc.Sampled)

	tc, ok = ParseTraceparent("00-11111111111111111111111111111111-2222222222222222-00")
	require.True(t, ok)
	assert.False(t, tc.Sampled)

	for _, invalid := range []string{
		"",
		"00-1111111111111111111111111111111-2222222222222222-01",   // short trace id
		"00-1111111111111111111111111111111g-2222222222222222-01",  // non-hex trace id
		"00-00000000000000000000000000000000-2222222222222222-01",  // zero trace id
		"00-11111111111111111111111111111111-0000000000000000-01",  // zero span id
		"00-11111111111111111111111111111111-2222222222222222",     // missing flags
		"ff-11111111111111111111111111111111-2222222222222222-01",  // unsupported version
		"00-11111111111111111111111111111111-2222222222222222-01-", // extra part
	} {
		_, ok = ParseTraceparent(invalid)
		assert.False(t, ok, "should reject %q", invalid)
	}
}

func TestTraceparentInheritance(t *testing.T) {
	globalTraceStarters = []traceStarter{&traceBuiltinStarter{}}
	defer func() { globalTraceStarters = nil }()

	incoming, ok := ParseTraceparent("00-11111111111111111111111111111111-2222222222222222-01")
	require.True(t, ok)

	// a root started with an incoming context joins that trace, keeps the
	// parent span id and the caller's sampling decision
	_, root := GetTracer().Start(WithIncomingTraceContext(t.Context(), incoming), "http")
	assert.Equal(t, incoming.TraceID, root.traceID)
	assert.Equal(t, incoming.SpanID, root.parentSpanID)
	assert.True(t, root.sampled)

	// the root's own traceparent round-trips through the parser, and a child
	// started with it stays in the same trace
	tp := root.Traceparent()
	require.NotEmpty(t, tp)
	parsed, ok := ParseTraceparent(tp)
	require.True(t, ok)
	assert.Equal(t, root.traceID, parsed.TraceID)
	assert.Equal(t, root.spanID, parsed.SpanID)
	assert.True(t, parsed.Sampled)

	_, child := GetTracer().Start(WithIncomingTraceContext(t.Context(), parsed), "child")
	assert.Equal(t, root.traceID, child.traceID)
	assert.Equal(t, root.spanID, child.parentSpanID)
	assert.True(t, child.sampled)
}

func TestTraceparentUnsampled(t *testing.T) {
	globalTraceStarters = []traceStarter{&traceBuiltinStarter{}}
	defer func() { globalTraceStarters = nil }()

	// an unsampled incoming context still joins the trace, but the spans are
	// not marked as sampled (the caller decided not to export)
	incoming, ok := ParseTraceparent("00-11111111111111111111111111111111-2222222222222222-00")
	require.True(t, ok)
	_, root := GetTracer().Start(WithIncomingTraceContext(t.Context(), incoming), "http")
	assert.Equal(t, incoming.TraceID, root.traceID)
	assert.False(t, root.sampled)
	assert.Contains(t, root.Traceparent(), "-00")
}
