// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !otel

package gtprof

import (
	"context"
	"time"
)

// OTELConfig is a stub when OTEL is not enabled
type OTELConfig struct {
	Enabled        bool
	ServiceName    string
	TracesEndpoint string
	Headers        map[string]string
	Insecure       bool
	Compression    string
	Timeout        time.Duration
}

// EnableOTELTracer is a no-op when OTEL build tag is not present
func EnableOTELTracer(config *OTELConfig) error {
	return nil
}

// ShutdownOTELTracer is a no-op when OTEL build tag is not present
func ShutdownOTELTracer(ctx context.Context) error {
	return nil
}
