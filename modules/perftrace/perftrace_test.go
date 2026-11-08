// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package perftrace

import (
	"testing"
	"time"

	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestListRoutes(t *testing.T) {
	routes = map[string]*routeData{}
	mk := func(route string, ms float64, longRunning bool) {
		add(&api.PerfTraceRecord{Time: time.Now(), Route: route, DurationMs: ms, LongRunning: longRunning, SQLCount: 1})
	}
	mk("/issues", 1500, false)
	mk("/issues", 2500, false)
	mk("/ws", 5000, true)
	mk("/fast", 10, false)

	ret := ListRoutes(Query{MinDuration: time.Second, ExcludeLongRunning: true})
	assert.Len(t, ret, 1)
	assert.Equal(t, "/issues", ret[0].Route)
	assert.EqualValues(t, 2, ret[0].Count)
	assert.InDelta(t, 2500, ret[0].P95DurationMs, 1)
	assert.Len(t, ret[0].Records, 2)

	ret = ListRoutes(Query{MinDuration: time.Second})
	assert.Len(t, ret, 2)
	assert.Equal(t, "/ws", ret[0].Route)
	assert.True(t, ret[0].LongRunning)
}
