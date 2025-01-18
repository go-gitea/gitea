// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/util"
)

type contextKey struct {
	name string
}

var ContextKeySpan = &contextKey{"span"}

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
	}

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
	ctx = context.WithValue(ctx, ContextKeySpan, ts)
	return ctx, ts
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
	ts, _ := ctx.Value(ContextKeySpan).(*TraceSpan)
	return ts
}
