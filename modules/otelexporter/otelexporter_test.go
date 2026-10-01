// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package otelexporter

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTrace() *OtelTrace {
	return &OtelTrace{ResourceSpans: []*OtelResourceSpan{{
		Resource: &OtelResource{Attributes: []*OtelAttribute{
			{Key: "service.name", Value: OtelAttributeStringValue{StringValue: "gitea"}},
		}},
		ScopeSpans: []*OtelScopeSpan{{
			Scope: &OtelScope{Name: "gitea-server", Version: "test"},
			Spans: []*OtelSpan{{
				TraceID:           "11111111111111111111111111111111",
				SpanID:            "2222222222222222",
				Name:              "http",
				Kind:              2,
				StartTimeUnixNano: "1700000000000000000",
				EndTimeUnixNano:   "1700000001000000000",
				Status:            2,
				StatusMessage:     "boom",
				Attributes: []*OtelAttribute{
					{Key: "http.response.status_code", Value: OtelAttributeIntValue{IntValue: "500"}},
					{Key: "flag", Value: OtelAttributeBoolValue{BoolValue: true}},
					{Key: "ratio", Value: OtelAttributeDoubleValue{DoubleValue: 0.5}},
				},
			}},
		}},
	}}}
}

func TestExportTracePostsOTLPJSON(t *testing.T) {
	var gotTrace OtelTrace
	var gotEncoding string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotEncoding = req.Header.Get("Content-Encoding")
		gotAuth = req.Header.Get("Authorization")
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		if gotEncoding == "gzip" {
			zr, err := gzip.NewReader(bytes.NewReader(body))
			require.NoError(t, err)
			body, err = io.ReadAll(zr)
			require.NoError(t, err)
		}
		require.NoError(t, json.Unmarshal(body, &gotTrace))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := Init(server.URL, map[string]string{"Authorization": "Bearer test"}, time.Second, true, false)
	defer StopDefault(time.Second)
	e.ExportTrace(testTrace())

	require.Eventually(t, func() bool { return atomic.LoadUint64(&e.Exported) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, uint64(0), atomic.LoadUint64(&e.Dropped))
	assert.Equal(t, "gzip", gotEncoding)
	assert.Equal(t, "Bearer test", gotAuth)
	require.Len(t, gotTrace.ResourceSpans, 1)
	require.Len(t, gotTrace.ResourceSpans[0].ScopeSpans[0].Spans, 1)
	span := gotTrace.ResourceSpans[0].ScopeSpans[0].Spans[0]
	assert.Equal(t, "http", span.Name)
	assert.Equal(t, 2, span.Kind)
	// OTLP JSON decodes the attribute value unions into generic maps
	require.Len(t, span.Attributes, 3)
	assert.Equal(t, map[string]any{"intValue": "500"}, span.Attributes[0].Value)
	assert.Equal(t, map[string]any{"boolValue": true}, span.Attributes[1].Value)
	assert.Equal(t, "gitea", gotTrace.ResourceSpans[0].Resource.Attributes[0].Value.(map[string]any)["stringValue"])
}

func TestExportTraceDropsOnEndpointFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	e := Init(server.URL, nil, time.Second, false, false)
	defer StopDefault(time.Second)
	e.ExportTrace(testTrace())
	require.Eventually(t, func() bool { return atomic.LoadUint64(&e.Dropped) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, uint64(0), atomic.LoadUint64(&e.Exported))
}

func TestExportTraceNeverBlocksWhenQueueIsFull(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(release)

	e := Init(server.URL, nil, 10*time.Second, false, false)
	defer StopDefault(time.Second)
	// the worker is stuck on the first trace; the rest fill the queue and one
	// more export must drop immediately instead of blocking
	e.ExportTrace(testTrace())
	for i := 0; i < queueSize; i++ {
		e.ExportTrace(testTrace())
	}
	done := make(chan struct{})
	go func() {
		e.ExportTrace(testTrace())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ExportTrace blocked while the queue was full")
	}
	assert.Equal(t, uint64(1), atomic.LoadUint64(&e.Dropped))
}

func TestStopDefaultDrainsQueue(t *testing.T) {
	var received atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := Init(server.URL, nil, time.Second, false, false)
	for i := 0; i < 3; i++ {
		e.ExportTrace(testTrace())
	}
	StopDefault(5 * time.Second)
	assert.Equal(t, int64(3), received.Load())
	assert.Equal(t, uint64(3), atomic.LoadUint64(&e.Exported))
}
