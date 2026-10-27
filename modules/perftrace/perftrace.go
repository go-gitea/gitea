// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package perftrace keeps the recent slow requests reported by the builtin tracer, grouped by route.
// The data is process-local and best-effort: with several instances each node only sees its own slow requests.
package perftrace

import (
	"slices"
	"strings"
	"sync"
	"time"

	"gitea.dev/modules/gtprof"
	api "gitea.dev/modules/structs"
)

const (
	maxRoutes          = 200
	maxRecordsPerRoute = 20
)

type routeData struct {
	count   int64
	maxMs   float64
	records []*api.PerfTraceRecord
}

var (
	mu     sync.Mutex
	routes = map[string]*routeData{}
)

// Enable makes the builtin tracer collect requests slower than the threshold, 0 disables it
func Enable(threshold time.Duration) {
	gtprof.EnableBuiltinTracer(threshold)
	gtprof.SetSlowTraceHandler(func(root *gtprof.TraceSpan) { add(newRecord(root)) })
}

func toMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func newRecord(root *gtprof.TraceSpan) *api.PerfTraceRecord {
	rec := &api.PerfTraceRecord{Time: time.Now()}
	rec.Root = convertSpan(rec, root)
	rec.DurationMs = rec.Root.DurationMs
	rec.Route = rec.Root.Attributes[gtprof.TraceAttrHTTPRoute]
	_, rec.LongRunning = rec.Root.Attributes[gtprof.TraceAttrLongRunning]
	return rec
}

func convertSpan(rec *api.PerfTraceRecord, s *gtprof.TraceSpan) *api.PerfTraceSpan {
	ret := &api.PerfTraceSpan{Name: s.Name(), DurationMs: toMs(s.Duration()), Attributes: s.Attributes()}
	switch ret.Name {
	case gtprof.TraceSpanDatabase:
		rec.SQLCount++
		rec.SQLDurationMs += ret.DurationMs
	case gtprof.TraceSpanGitRun:
		rec.GitCount++
		rec.GitDurationMs += ret.DurationMs
	}
	for _, c := range s.Children() {
		ret.Children = append(ret.Children, convertSpan(rec, c))
	}
	return ret
}

func add(rec *api.PerfTraceRecord) {
	mu.Lock()
	defer mu.Unlock()
	d := routes[rec.Route]
	if d == nil {
		if len(routes) >= maxRoutes { // drop the route whose newest record is the oldest
			var oldest string
			var oldestTime time.Time
			for k, v := range routes {
				if t := v.records[len(v.records)-1].Time; oldest == "" || t.Before(oldestTime) {
					oldest, oldestTime = k, t
				}
			}
			delete(routes, oldest)
		}
		d = &routeData{}
		routes[rec.Route] = d
	}
	d.count++
	d.maxMs = max(d.maxMs, rec.DurationMs)
	d.records = append(d.records, rec)
	if len(d.records) > maxRecordsPerRoute {
		d.records = d.records[len(d.records)-maxRecordsPerRoute:]
	}
}

// Query filters the result of ListRoutes
type Query struct {
	MinDuration        time.Duration
	Route              string // substring match
	ExcludeLongRunning bool
	Since              time.Time
}

// ListRoutes returns per-route aggregations, the slowest P95 first
func ListRoutes(q Query) []*api.PerfTraceRoute {
	mu.Lock()
	defer mu.Unlock()

	minMs := toMs(q.MinDuration)
	ret := []*api.PerfTraceRoute{}
	for route, d := range routes {
		if q.Route != "" && !strings.Contains(route, q.Route) {
			continue
		}
		var recs []*api.PerfTraceRecord
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
		ret = append(ret, &api.PerfTraceRoute{
			Route:         route,
			LongRunning:   recs[0].LongRunning,
			Count:         d.count,
			MaxDurationMs: d.maxMs,
			P95DurationMs: durs[min(len(durs)-1, len(durs)*95/100)],
			LastTime:      recs[len(recs)-1].Time,
			Records:       recs,
		})
	}
	slices.SortFunc(ret, func(a, b *api.PerfTraceRoute) int {
		if a.P95DurationMs != b.P95DurationMs {
			if a.P95DurationMs > b.P95DurationMs {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Route, b.Route)
	})
	return ret
}
