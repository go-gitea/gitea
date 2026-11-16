// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"
)

// BenchmarkInsertEvent measures the synchronous database cost added when audit
// recording is enabled. Keep it separate from router benchmarks so it remains
// comparable across changes to request handling.
func BenchmarkInsertEvent(b *testing.B) {
	if err := unittest.PrepareTestDatabase(); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := &Event{
			Action:        UserPassword,
			ActorID:       1,
			ActorName:     "actor",
			ScopeType:     ScopeUser,
			ScopeID:       2,
			ScopeName:     "scope",
			Origin:        OriginUI,
			Metadata:      `{"source":"benchmark"}`,
			TimestampUnix: timeutil.TimeStamp(i + 1),
		}
		if err := InsertEvent(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}
