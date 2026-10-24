// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTraceRecordStore(t *testing.T) {
	globalTraceStore = &traceStore{routes: map[string]*traceRouteData{}}

	mk := func(route string, d time.Duration) {
		_, root := (&Tracer{starters: []traceStarter{}}).Start(t.Context(), TraceSpanHTTP)
		root.SetAttributeString(TraceAttrHTTPRoute, route)
		db := &TraceSpan{name: TraceSpanDatabase, startTime: root.startTime, endTime: root.startTime.Add(d / 2)}
		root.children = append(root.children, db)
		root.endTime = root.startTime.Add(d)
		globalTraceStore.add(newTraceRecord(root))
	}
	mk("/{owner}/{repo}/issues", 1500*time.Millisecond)
	mk("/{owner}/{repo}/issues", 2500*time.Millisecond)
	mk("/user/events", 5*time.Second)
	mk("/fast", 10*time.Millisecond)

	stats := GetTraceRouteStats(TraceQuery{MinDuration: time.Second, ExcludeLongRunning: true})
	assert.Len(t, stats, 1)
	assert.Equal(t, "/{owner}/{repo}/issues", stats[0].Route)
	assert.EqualValues(t, 2, stats[0].Count)
	assert.InDelta(t, 2500.0, stats[0].P95DurationMs, 1)
	assert.Len(t, stats[0].Records, 2)
	assert.Equal(t, 1, stats[0].Records[0].SQLCount)
	assert.InDelta(t, 750.0, stats[0].Records[0].SQLDurationMs, 1)

	stats = GetTraceRouteStats(TraceQuery{MinDuration: time.Second})
	assert.Len(t, stats, 2)
	assert.Equal(t, "/user/events", stats[0].Route)
	assert.True(t, stats[0].LongRunning)
}
