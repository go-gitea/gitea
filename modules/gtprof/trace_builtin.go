// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/tailmsg"
)

type traceBuiltinStarter struct{}

type traceBuiltinSpan struct {
	ts *TraceSpan

	internalSpanIdx int
}

func (t *traceBuiltinSpan) toString(out *strings.Builder, indent int) {
	t.ts.mu.RLock()
	defer t.ts.mu.RUnlock()

	out.WriteString(strings.Repeat(" ", indent))
	out.WriteString(t.ts.name)
	if t.ts.endTime.IsZero() {
		out.WriteString(" duration: (not ended)")
	} else {
		out.WriteString(fmt.Sprintf(" duration: %.4fs", t.ts.endTime.Sub(t.ts.startTime).Seconds()))
	}
	out.WriteString("\n")
	for _, a := range t.ts.attributes {
		out.WriteString(strings.Repeat(" ", indent+2))
		out.WriteString(a.Key)
		out.WriteString(": ")
		out.WriteString(a.Value.AsString())
		out.WriteString("\n")
	}
	for _, c := range t.ts.children {
		span := c.internalSpans[t.internalSpanIdx].(*traceBuiltinSpan)
		span.toString(out, indent+2)
	}
}

func (t *traceBuiltinSpan) end() {
	if t.ts.parent == nil {
		// FIXME: debug purpose only
		// FIXME: it should distinguish between http response network lag and actual processing time
		if len(t.ts.children) > 3 || t.ts.endTime.Sub(t.ts.startTime) > 100*time.Millisecond {
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
