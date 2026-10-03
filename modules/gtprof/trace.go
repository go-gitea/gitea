// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"gitea.dev/modules/util"
)

type contextKey struct {
	name string
}

var contextKeySpan = &contextKey{"span"}

type traceStarter interface {
	start(ctx context.Context, traceSpan *TraceSpan, internalSpanIdx int) (context.Context, traceSpanInternal)
}

type traceSpanInternal interface {
	addEvent(name string, cfg *EventConfig)
	recordError(err error, cfg *EventConfig)
	end()
}

type TraceSpan struct {
	// immutable
	parent           *TraceSpan
	traceID          [16]byte // the trace this span belongs to, from the parent span or the incoming W3C "traceparent" header, or random for a new root
	spanID           [8]byte
	parentSpanID     [8]byte // zero for a root span without an incoming trace context
	sampled          bool    // whether this trace should be exported (the caller's sampling decision for an inherited trace)
	internalSpans    []traceSpanInternal
	internalContexts []context.Context

	// mutable, must be protected by mutex
	mu         sync.RWMutex
	name       string
	statusCode uint32
	statusDesc string
	startTime  time.Time
	endTime    time.Time
	attributes []*TraceAttribute
	children   []*TraceSpan
}

type TraceAttribute struct {
	Key   string
	Value TraceValue
}

type TraceValue struct {
	v any
}

func (t *TraceValue) AsString() string {
	return fmt.Sprint(t.v)
}

func (t *TraceValue) AsInt64() int64 {
	v, _ := util.ToInt64(t.v)
	return v
}

func (t *TraceValue) AsFloat64() float64 {
	v, _ := util.ToFloat64(t.v)
	return v
}

var globalTraceStarters []traceStarter

type Tracer struct {
	starters []traceStarter
}

func (s *TraceSpan) SetName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
}

func (s *TraceSpan) SetStatus(code uint32, desc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCode, s.statusDesc = code, desc
}

func (s *TraceSpan) AddEvent(name string, options ...EventOption) {
	cfg := eventConfigFromOptions(options...)
	for _, tsp := range s.internalSpans {
		tsp.addEvent(name, cfg)
	}
}

func (s *TraceSpan) RecordError(err error, options ...EventOption) {
	cfg := eventConfigFromOptions(options...)
	for _, tsp := range s.internalSpans {
		tsp.recordError(err, cfg)
	}
}

func (s *TraceSpan) SetAttributeString(key, value string) *TraceSpan {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attributes = append(s.attributes, &TraceAttribute{Key: key, Value: TraceValue{v: value}})
	return s
}

// SetAttribute records an attribute of any basic type (string, int, int64, float64, bool),
// for exporters that map the value to its OTel type.
func (s *TraceSpan) SetAttribute(key string, value any) *TraceSpan {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attributes = append(s.attributes, &TraceAttribute{Key: key, Value: TraceValue{v: value}})
	return s
}

// Traceparent returns the W3C "traceparent" value for this span, to propagate
// the current trace context to an outgoing request. It returns an empty string
// when the span has no identity (not started through Tracer.Start).
func (s *TraceSpan) Traceparent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.traceID == ([16]byte{}) || s.spanID == ([8]byte{}) {
		return ""
	}
	flags := "00"
	if s.sampled {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", hex.EncodeToString(s.traceID[:]), hex.EncodeToString(s.spanID[:]), flags)
}

func (t *Tracer) Start(ctx context.Context, spanName string) (context.Context, *TraceSpan) {
	starters := t.starters
	if starters == nil {
		starters = globalTraceStarters
	}
	ts := &TraceSpan{name: spanName, startTime: time.Now()}
	parentSpan := GetContextSpan(ctx)
	if parentSpan != nil {
		parentSpan.mu.Lock()
		parentSpan.children = append(parentSpan.children, ts)
		parentSpan.mu.Unlock()
		ts.parent = parentSpan
		ts.traceID = parentSpan.traceID
		ts.parentSpanID = parentSpan.spanID
		ts.sampled = parentSpan.sampled
	} else if tc, ok := GetIncomingTraceContext(ctx); ok {
		// the caller propagated a W3C "traceparent": join that trace, keep its
		// sampling decision, and continue it on outgoing requests. The trace ID
		// is never derived from an issue number or any business identifier.
		ts.traceID = tc.TraceID
		ts.parentSpanID = tc.SpanID
		ts.sampled = tc.Sampled
	}
	if ts.traceID == ([16]byte{}) {
		ts.traceID = randomTraceID()
	}
	if ts.spanID == ([8]byte{}) {
		ts.spanID = randomSpanID()
	}

	// FIXME: this ctx handling is not right. The returned ctx should inherit the ctx passed in, but not from span's internal contexts
	// The returned ctx only needs to inherit the values of the internal contexts of spans
	parentCtx := ctx
	for internalSpanIdx, tsp := range starters {
		var internalSpan traceSpanInternal
		if parentSpan != nil {
			parentCtx = parentSpan.internalContexts[internalSpanIdx]
		}
		ctx, internalSpan = tsp.start(parentCtx, ts, internalSpanIdx)
		ts.internalContexts = append(ts.internalContexts, ctx)
		ts.internalSpans = append(ts.internalSpans, internalSpan)
	}
	ctx = context.WithValue(ctx, contextKeySpan, ts)
	return ctx, ts
}

type mutableContext interface {
	context.Context
	SetContextValue(key, value any)
	GetContextValue(key any) any
}

// StartInContext starts a trace span in Gitea's mutable context (usually the web request context).
// Due to the design limitation of Gitea's web framework, it can't use `context.WithValue` to bind a new span into a new context.
// So here we use our "reqctx" framework to achieve the same result: web request context could always see the latest "span".
func (t *Tracer) StartInContext(ctx mutableContext, spanName string) (*TraceSpan, func()) {
	curTraceSpan := GetContextSpan(ctx)
	_, newTraceSpan := GetTracer().Start(ctx, spanName)
	ctx.SetContextValue(contextKeySpan, newTraceSpan)
	return newTraceSpan, func() {
		newTraceSpan.End()
		ctx.SetContextValue(contextKeySpan, curTraceSpan)
	}
}

func (s *TraceSpan) End() {
	s.mu.Lock()
	s.endTime = time.Now()
	s.mu.Unlock()

	for _, tsp := range s.internalSpans {
		tsp.end()
	}
}

func GetTracer() *Tracer {
	return &Tracer{}
}

func GetContextSpan(ctx context.Context) *TraceSpan {
	ts, _ := ctx.Value(contextKeySpan).(*TraceSpan)
	return ts
}
