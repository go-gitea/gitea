// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package otelexporter posts traces to an OpenTelemetry collector over the
// OTLP/HTTP JSON protocol (https://opentelemetry.io/docs/specs/otlp/). It is a
// small, dependency-free exporter: no OTel SDK is linked, so disabled builds
// keep their binary size, and the payload is the officially supported OTLP
// JSON wire format.
package otelexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"gitea.dev/modules/json"
)

// OtelAttributeStringValue is the OTLP JSON encoding of a string attribute.
type OtelAttributeStringValue struct {
	StringValue string `json:"stringValue"`
}

// OtelAttributeIntValue is the OTLP JSON encoding of a signed-int attribute.
type OtelAttributeIntValue struct {
	IntValue string `json:"intValue"`
}

// OtelAttributeDoubleValue is the OTLP JSON encoding of a double attribute.
type OtelAttributeDoubleValue struct {
	DoubleValue float64 `json:"doubleValue"`
}

// OtelAttributeBoolValue is the OTLP JSON encoding of a bool attribute.
type OtelAttributeBoolValue struct {
	BoolValue bool `json:"boolValue"`
}

type OtelAttribute struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type OtelScope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type OtelSpan struct {
	TraceID           string           `json:"traceId"`
	SpanID            string           `json:"spanId"`
	ParentSpanID      string           `json:"parentSpanId,omitempty"`
	Name              string           `json:"name"`
	Kind              int              `json:"kind"`
	StartTimeUnixNano string           `json:"startTimeUnixNano"`
	EndTimeUnixNano   string           `json:"endTimeUnixNano"`
	Status            int              `json:"status,omitempty"` // OTLP SpanStatus code: 0 unset, 2 error
	StatusMessage     string           `json:"message,omitempty"`
	Attributes        []*OtelAttribute `json:"attributes,omitempty"`
}

type OtelScopeSpan struct {
	Scope *OtelScope  `json:"scope"`
	Spans []*OtelSpan `json:"spans"`
}

type OtelResource struct {
	Attributes []*OtelAttribute `json:"attributes,omitempty"`
}

type OtelResourceSpan struct {
	Resource   *OtelResource    `json:"resource"`
	ScopeSpans []*OtelScopeSpan `json:"scopeSpans"`
}

type OtelTrace struct {
	ResourceSpans []*OtelResourceSpan `json:"resourceSpans"`
}

// queueSize bounds the number of pending traces. One trace is one request's
// span tree, so a few hundred covers long bursts of slow requests.
const queueSize = 1024

// Exporter posts OTLP/HTTP JSON trace payloads to one endpoint.
type Exporter struct {
	endpoint    string
	headers     map[string]string
	timeout     time.Duration
	gzipEnabled bool
	httpClient  *http.Client

	queue   chan *OtelTrace
	done    chan struct{}
	closing atomic.Bool

	// Exported/Dropped are counters for the delivery outcome, for the admin
	// diagnostics. Exported counts traces the endpoint accepted (a 2xx means
	// fully accepted per OTLP); Dropped counts every trace this exporter did
	// not deliver (full queue, endpoint error, shutdown). They are not
	// evidence that a stored trace is queryable, only that it was sent.
	Exported uint64
	Dropped  uint64
}

// Init starts the exporter and its single delivery worker, and remembers it as
// the process default.
func Init(endpoint string, headers map[string]string, timeout time.Duration, gzipEnabled, tlsInsecure bool) *Exporter {
	e := &Exporter{
		endpoint:    endpoint,
		headers:     headers,
		timeout:     timeout,
		gzipEnabled: gzipEnabled,
		queue:       make(chan *OtelTrace, queueSize),
		done:        make(chan struct{}),
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if tlsInsecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // an explicit opt-out for self-hosted collectors
	}
	e.httpClient = &http.Client{Timeout: timeout, Transport: transport}
	go e.worker()
	defaultExporter.Store(e)
	return e
}

var defaultExporter atomic.Pointer[Exporter]

// Default returns the process default exporter, or nil when OTLP export is disabled.
func Default() *Exporter { return defaultExporter.Load() }

// StopDefault stops accepting new traces and drains the queue within the
// timeout. Traces still pending after the timeout are dropped and counted.
func StopDefault(timeout time.Duration) {
	e := Default()
	if e == nil {
		return
	}
	if !e.closing.CompareAndSwap(false, true) {
		return
	}
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			// do not wait for the worker: the remaining items count as dropped
			return
		default:
		}
		if len(e.queue) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ExportTrace enqueues a trace for delivery. It never blocks: when the queue
// is full or the exporter is stopping, the trace is dropped and counted.
func (e *Exporter) ExportTrace(t *OtelTrace) {
	if e.closing.Load() {
		atomic.AddUint64(&e.Dropped, 1)
		return
	}
	select {
	case e.queue <- t:
	default:
		atomic.AddUint64(&e.Dropped, 1)
	}
}

func (e *Exporter) worker() {
	for t := range e.queue {
		if !e.post(t) {
			atomic.AddUint64(&e.Dropped, 1)
		}
	}
}

func (e *Exporter) post(t *OtelTrace) bool {
	body, err := json.Marshal(t)
	if err != nil {
		return false
	}
	contentType := "application/json"
	payload := bytes.NewReader(body)
	if e.gzipEnabled {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err = zw.Write(body); err != nil || zw.Close() != nil {
			return false
		}
		payload = bytes.NewReader(buf.Bytes())
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, payload)
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", contentType)
	if e.gzipEnabled {
		req.Header.Set("Content-Encoding", "gzip")
	}
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	// OTLP: a 2xx means fully accepted; anything else (throttle, partial
	// success, auth failure) is not retried by this minimal exporter and is
	// counted as dropped.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		atomic.AddUint64(&e.Exported, 1)
		return true
	}
	return false
}
