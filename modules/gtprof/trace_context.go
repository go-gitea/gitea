// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// TraceContext is the parsed W3C "traceparent" header
// (https://www.w3.org/TR/trace-context/), the standard way to propagate a
// trace across a process boundary. Gitea only reads and writes version-00
// values and never derives a trace ID from a business identifier.
type TraceContext struct {
	TraceID [16]byte
	SpanID  [8]byte
	Sampled bool
}

var contextKeyIncomingTrace = &contextKey{"incoming-trace"}

// WithIncomingTraceContext stores a valid incoming trace context in the
// context, so the next root span started in it joins the propagated trace.
func WithIncomingTraceContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, contextKeyIncomingTrace, tc)
}

// GetIncomingTraceContext reads the trace context set by WithIncomingTraceContext.
func GetIncomingTraceContext(ctx context.Context) (TraceContext, bool) {
	tc, ok := ctx.Value(contextKeyIncomingTrace).(TraceContext)
	return tc, ok
}

// ParseTraceparent parses a W3C version-00 "traceparent" header value strictly:
// "00-<32 lowercase hex>-<16 lowercase hex>-<2 hex flags>". Zero trace or span
// IDs are invalid; the flags byte may be zero.
func ParseTraceparent(header string) (TraceContext, bool) {
	parts := strings.Split(header, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return TraceContext{}, false
	}
	var tc TraceContext
	if !decodeFixedHex(tc.TraceID[:], parts[1]) || !decodeFixedHex(tc.SpanID[:], parts[2]) {
		return TraceContext{}, false
	}
	var flags [1]byte
	if !decodeHexFixed(flags[:], parts[3]) {
		return TraceContext{}, false
	}
	tc.Sampled = flags[0]&0x01 != 0
	return tc, true
}

// decodeFixedHex decodes exactly len(dst) bytes of lowercase hex and rejects
// all-zero values (the W3C spec invalidates zero trace/span IDs).
func decodeFixedHex(dst []byte, s string) bool {
	if !decodeHexFixed(dst, s) {
		return false
	}
	for _, c := range dst {
		if c != 0 {
			return true
		}
	}
	return false
}

// decodeHexFixed decodes exactly len(dst) bytes of lowercase hex.
func decodeHexFixed(dst []byte, s string) bool {
	if len(s) != len(dst)*2 || s != strings.ToLower(s) {
		return false
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return false
	}
	copy(dst, b)
	return true
}

// randomBytes reads cryptographically secure random bytes. crypto/rand.Read
// never fails in practice, and an all-zero ID is invalid by the W3C spec, so
// it retries (an astronomically unlikely event) instead of emitting it.
func randomTraceID() [16]byte {
	for {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			panic("gtprof: crypto/rand.Read failed: " + err.Error())
		}
		if id != ([16]byte{}) {
			return id
		}
	}
}

func randomSpanID() [8]byte {
	for {
		var id [8]byte
		if _, err := rand.Read(id[:]); err != nil {
			panic("gtprof: crypto/rand.Read failed: " + err.Error())
		}
		if id != ([8]byte{}) {
			return id
		}
	}
}
