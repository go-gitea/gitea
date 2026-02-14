// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"strconv"

	"code.gitea.io/gitea/modules/otelexporter"
)

func otelToSpan(traceID string, t *traceBuiltinSpan, scopeSpan *otelexporter.OtelScopeSpan) {
	t.ts.mu.RLock()
	defer t.ts.mu.RUnlock()

	span := &otelexporter.OtelSpan{
		TraceID:           traceID,
		SpanID:            t.ts.id,
		Name:              t.ts.name,
		StartTimeUnixNano: strconv.FormatInt(t.ts.startTime.UnixNano(), 10),
		EndTimeUnixNano:   strconv.FormatInt(t.ts.endTime.UnixNano(), 10),
		Kind:              2,
	}

	if t.ts.parent != nil {
		span.ParentSpanID = t.ts.parent.id
	}

	scopeSpan.Spans = append(scopeSpan.Spans, span)

	for _, a := range t.ts.attributes {
		var otelVal any
		if a.Value.IsString() {
			otelVal = otelexporter.OtelAttributeStringValue{StringValue: a.Value.AsString()}
		} else {
			otelVal = otelexporter.OtelAttributeIntValue{IntValue: strconv.FormatInt(a.Value.AsInt64(), 10)}
		}
		span.Attributes = append(span.Attributes, &otelexporter.OtelAttribute{Key: a.Key, Value: otelVal})
	}

	for _, c := range t.ts.children {
		child := c.internalSpans[t.internalSpanIdx].(*traceBuiltinSpan)
		otelToSpan(traceID, child, scopeSpan)
	}
}

func otelRecordTrace(t *traceBuiltinSpan) {
	exporter := otelexporter.GetDefaultOtelExporter()
	if exporter == nil {
		return
	}
	opts := tracerOptions.Load()
	scopeSpan := &otelexporter.OtelScopeSpan{
		Scope: &otelexporter.OtelScope{Name: "gitea-server", Version: opts.AppVer},
	}

	traceID := GetTracer().randomHexForBytes(16)
	otelToSpan(traceID, t, scopeSpan)

	resSpans := otelexporter.OtelResourceSpan{
		Resource: &otelexporter.OtelResource{
			Attributes: []*otelexporter.OtelAttribute{
				{Key: "service.name", Value: otelexporter.OtelAttributeStringValue{StringValue: opts.ServiceName}},
			},
		},
		ScopeSpans: []*otelexporter.OtelScopeSpan{scopeSpan},
	}

	otelTrace := &otelexporter.OtelTrace{ResourceSpans: []*otelexporter.OtelResourceSpan{&resSpans}}
	exporter.ExportTrace(otelTrace)
}
