// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	TraceAttrLongRunning = "http.long_running"

	traceStoreMaxRoutes   = 200
	traceStoreRoutePerKey = 20
)

// longRunningRouteHints are route-pattern fragments of endpoints which are slow by design (streams, git transfer, downloads)
var longRunningRouteHints = []string{
	"/events", "/info/refs", "/git-upload-pack", "/git-receive-pack", "/git-upload-archive", "/objects/",
	"/lfs", "/raw/", "/media/", "/archive/", "/attachments/", "/releases/download/",
}

func isLongRunningRoute(route string) bool {
	return slices.ContainsFunc(longRunningRouteHints, func(h string) bool { return strings.Contains(route, h) })
}

// TraceSpanRecord is the structured (JSON-friendly) form of a finished span
type TraceSpanRecord struct {
	Name       string             `json:"name"`
	DurationMs float64            `json:"duration_ms"`
	Attributes map[string]string  `json:"attributes,omitempty"`
	Children   []*TraceSpanRecord `json:"children,omitempty"`
}

// TraceRecord is a slow root span with its summary counters
type TraceRecord struct {
	Time          time.Time        `json:"time"`
	Route         string           `json:"route"`
	DurationMs    float64          `json:"duration_ms"`
	LongRunning   bool             `json:"long_running"`
	SQLCount      int              `json:"sql_count"`
	SQLDurationMs float64          `json:"sql_duration_ms"`
	GitCount      int              `json:"git_count"`
	GitDurationMs float64          `json:"git_duration_ms"`
	Root          *TraceSpanRecord `json:"root"`
}

// TraceRouteStat is the aggregation of the slow records of one route
type TraceRouteStat struct {
	Route         string         `json:"route"`
	LongRunning   bool           `json:"long_running"`
	Count         int64          `json:"count"`
	MaxDurationMs float64        `json:"max_duration_ms"`
	P95DurationMs float64        `json:"p95_duration_ms"`
	LastTime      time.Time      `json:"last_time"`
	Records       []*TraceRecord `json:"records"`
}

type traceRouteData struct {
	count   int64
	maxMs   float64
	records []*TraceRecord
}

type traceStore struct {
	mu     sync.Mutex
	routes map[string]*traceRouteData
}

var globalTraceStore = &traceStore{routes: map[string]*traceRouteData{}}

func (s *TraceSpan) toRecord(rec *TraceRecord) *TraceSpanRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dur := float64(s.endTime.Sub(s.startTime)) / float64(time.Millisecond)
	sr := &TraceSpanRecord{Name: s.name, DurationMs: dur}
	if len(s.attributes) > 0 {
		sr.Attributes = make(map[string]string, len(s.attributes))
		for _, a := range s.attributes {
			sr.Attributes[a.Key] = a.Value.AsString()
		}
	}
	switch s.name {
	case TraceSpanDatabase:
		rec.SQLCount++
		rec.SQLDurationMs += dur
	case TraceSpanGitRun:
		rec.GitCount++
		rec.GitDurationMs += dur
	}
	for _, c := range s.children {
		sr.Children = append(sr.Children, c.toRecord(rec))
	}
	return sr
}

// newTraceRecord converts an ended root span into a record
func newTraceRecord(root *TraceSpan) *TraceRecord {
	rec := &TraceRecord{Time: time.Now()}
	rec.Root = root.toRecord(rec)
	rec.DurationMs = rec.Root.DurationMs
	rec.Route = rec.Root.Attributes[TraceAttrHTTPRoute]
	_, flagged := rec.Root.Attributes[TraceAttrLongRunning]
	rec.LongRunning = flagged || isLongRunningRoute(rec.Route)
	return rec
}

func (s *traceStore) add(rec *TraceRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.routes[rec.Route]
	if d == nil {
		if len(s.routes) >= traceStoreMaxRoutes { // drop the route whose newest record is the oldest
			var oldest string
			var oldestTime time.Time
			for k, v := range s.routes {
				if t := v.records[len(v.records)-1].Time; oldest == "" || t.Before(oldestTime) {
					oldest, oldestTime = k, t
				}
			}
			delete(s.routes, oldest)
		}
		d = &traceRouteData{}
		s.routes[rec.Route] = d
	}
	d.count++
	d.maxMs = max(d.maxMs, rec.DurationMs)
	d.records = append(d.records, rec)
	if len(d.records) > traceStoreRoutePerKey {
		d.records = d.records[len(d.records)-traceStoreRoutePerKey:]
	}
}

// TraceQuery filters the result of GetTraceRouteStats
type TraceQuery struct {
	MinDuration        time.Duration
	Route              string // substring match
	ExcludeLongRunning bool
	Since              time.Time
}

// GetTraceRouteStats returns per-route aggregations, slowest P95 first
func GetTraceRouteStats(q TraceQuery) []*TraceRouteStat {
	s := globalTraceStore
	s.mu.Lock()
	defer s.mu.Unlock()

	minMs := float64(q.MinDuration) / float64(time.Millisecond)
	var stats []*TraceRouteStat
	for route, d := range s.routes {
		if q.Route != "" && !strings.Contains(route, q.Route) {
			continue
		}
		var recs []*TraceRecord
		var durs []float64
		for _, r := range d.records {
			if r.DurationMs < minMs || r.Time.Before(q.Since) || (q.ExcludeLongRunning && r.LongRunning) {
				continue
			}
			recs = append(recs, r)
			durs = append(durs, r.DurationMs)
		}
		if len(recs) == 0 {
			continue
		}
		slices.Sort(durs)
		stats = append(stats, &TraceRouteStat{
			Route:         route,
			LongRunning:   recs[0].LongRunning,
			Count:         d.count,
			MaxDurationMs: d.maxMs,
			P95DurationMs: durs[min(len(durs)-1, len(durs)*95/100)],
			LastTime:      recs[len(recs)-1].Time,
			Records:       recs,
		})
	}
	slices.SortFunc(stats, func(a, b *TraceRouteStat) int {
		if a.P95DurationMs != b.P95DurationMs {
			if a.P95DurationMs > b.P95DurationMs {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Route, b.Route)
	})
	return stats
}
