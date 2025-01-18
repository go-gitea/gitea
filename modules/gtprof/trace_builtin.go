// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/tailmsg"
)

type traceBuiltinStarter struct{}

type traceBuiltinSpan struct {
	ts *TraceSpan

	internalSpanIdx int
}

func (t *traceBuiltinSpan) addEvent(name string, cfg *EventConfig) {
	// No-op because builtin tracer doesn't need it.
	// In the future we might use it to mark the time point between backend logic and network response.
}

func (t *traceBuiltinSpan) recordError(err error, cfg *EventConfig) {
	// No-op because builtin tracer doesn't need it.
	// Actually Gitea doesn't handle err this way in most cases
}

func (t *traceBuiltinSpan) toString(out *strings.Builder, indent int) {
	t.ts.mu.RLock()
	defer t.ts.mu.RUnlock()

	out.WriteString(strings.Repeat(" ", indent))
	out.WriteString(t.ts.name)
	if t.ts.endTime.IsZero() {
		out.WriteString(" duration: (not ended)")
	} else {
		out.WriteString(fmt.Sprintf(" duration=%.4fs", t.ts.endTime.Sub(t.ts.startTime).Seconds()))
	}
	for _, a := range t.ts.attributes {
		out.WriteString(" ")
		out.WriteString(a.Key)
		out.WriteString("=")
		value := a.Value.AsString()
		if strings.ContainsAny(value, " \t\r\n") {
			quoted := false
			for _, c := range "\"'`" {
				if quoted = !strings.Contains(value, string(c)); quoted {
					value = string(c) + value + string(c)
					break
				}
			}
			if !quoted {
				value = fmt.Sprintf("%q", value)
			}
		}
		out.WriteString(value)
	}
	out.WriteString("\n")
	for _, c := range t.ts.children {
		span := c.internalSpans[t.internalSpanIdx].(*traceBuiltinSpan)
		span.toString(out, indent+2)
	}
}

func (t *traceBuiltinSpan) end() {
	if t.ts.parent == nil {
		// TODO: debug purpose only
		// TODO: it should distinguish between http response network lag and actual processing time
		threshold := time.Duration(traceBuiltinThreshold.Load())
		if threshold != 0 && t.ts.endTime.Sub(t.ts.startTime) > threshold {
			sb := &strings.Builder{}
			t.toString(sb, 0)
			tailmsg.GetManager().GetTraceRecorder().Record(sb.String())
		}
	}
}

func (t *traceBuiltinStarter) start(ctx context.Context, traceSpan *TraceSpan, internalSpanIdx int) (context.Context, traceSpanInternal) {
	return ctx, &traceBuiltinSpan{ts: traceSpan, internalSpanIdx: internalSpanIdx}
}

func init() {
	globalTraceStarters = append(globalTraceStarters, &traceBuiltinStarter{})
}

var traceBuiltinThreshold atomic.Int64

func EnableBuiltinTracer(threshold time.Duration) {
	traceBuiltinThreshold.Store(int64(threshold))
}
