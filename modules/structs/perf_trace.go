// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// PerfTraceSpan is a finished span of a slow request
type PerfTraceSpan struct {
	Name       string            `json:"name"`
	DurationMs float64           `json:"duration_ms"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Children   []*PerfTraceSpan  `json:"children,omitempty"`
}

// PerfTraceRecord is a slow request with its summary counters
type PerfTraceRecord struct {
	Time          time.Time      `json:"time"`
	Route         string         `json:"route"`
	DurationMs    float64        `json:"duration_ms"`
	LongRunning   bool           `json:"long_running"`
	SQLCount      int            `json:"sql_count"`
	SQLDurationMs float64        `json:"sql_duration_ms"`
	GitCount      int            `json:"git_count"`
	GitDurationMs float64        `json:"git_duration_ms"`
	Root          *PerfTraceSpan `json:"root"`
}

// PerfTraceRoute is the aggregation of the slow requests of one route
type PerfTraceRoute struct {
	Route         string             `json:"route"`
	LongRunning   bool               `json:"long_running"`
	Count         int64              `json:"count"`
	MaxDurationMs float64            `json:"max_duration_ms"`
	P95DurationMs float64            `json:"p95_duration_ms"`
	LastTime      time.Time          `json:"last_time"`
	Records       []*PerfTraceRecord `json:"records"`
}
