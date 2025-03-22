// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package otelexporter

import (
	"bytes"
	"net/http"
	"sync/atomic"

	"code.gitea.io/gitea/modules/json"
)

type OtelAttributeStringValue struct {
	StringValue string `json:"stringValue"`
}

type OtelAttributeIntValue struct {
	IntValue string `json:"intValue"`
}

type OtelAttribute struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type OtelResource struct {
	Attributes []*OtelAttribute `json:"attributes,omitempty"`
}

type OtelScope struct {
	Name       string           `json:"name"`
	Version    string           `json:"version"`
	Attributes []*OtelAttribute `json:"attributes,omitempty"`
}

type OtelSpan struct {
	TraceID           string           `json:"traceId"`
	SpanID            string           `json:"spanId"`
	ParentSpanID      string           `json:"parentSpanId,omitempty"`
	Name              string           `json:"name"`
	StartTimeUnixNano string           `json:"startTimeUnixNano"`
	EndTimeUnixNano   string           `json:"endTimeUnixNano"`
	Kind              int              `json:"kind"`
	Attributes        []*OtelAttribute `json:"attributes,omitempty"`
}

type OtelScopeSpan struct {
	Scope *OtelScope  `json:"scope"`
	Spans []*OtelSpan `json:"spans"`
}

type OtelResourceSpan struct {
	Resource   *OtelResource    `json:"resource"`
	ScopeSpans []*OtelScopeSpan `json:"scopeSpans"`
}

type OtelTrace struct {
	ResourceSpans []*OtelResourceSpan `json:"resourceSpans"`
}

type OtelExporter struct{}

func (e *OtelExporter) ExportTrace(trace *OtelTrace) {
	// TODO: use a async queue
	otelTraceJSON, err := json.Marshal(trace)
	if err == nil {
		_, _ = http.Post("http://localhost:4318/v1/traces", "application/json", bytes.NewReader(otelTraceJSON))
	}
}

var defaultOtelExporter atomic.Pointer[OtelExporter]

func GetDefaultOtelExporter() *OtelExporter {
	return defaultOtelExporter.Load()
}

func InitDefaultOtelExporter() {
	e := &OtelExporter{}
	defaultOtelExporter.Store(e)
}
