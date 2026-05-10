// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPlanLogUpdate(t *testing.T) {
	row := func(c string) *runnerv1.LogRow {
		return &runnerv1.LogRow{Time: timestamppb.New(time.Unix(0, 0)), Content: c}
	}
	a, b, c := row("a"), row("b"), row("c")

	cases := []struct {
		name         string
		rows         []*runnerv1.LogRow
		index, ack   int64
		noMore       bool
		wantNewRows  []*runnerv1.LogRow
		wantFinalize bool
		wantBail     bool
	}{
		{"fresh batch", []*runnerv1.LogRow{a, b, c}, 0, 0, false, []*runnerv1.LogRow{a, b, c}, false, false},
		{"fresh batch with NoMore finalizes", []*runnerv1.LogRow{a, b, c}, 0, 0, true, []*runnerv1.LogRow{a, b, c}, true, false},
		{"partial overlap trims acked rows", []*runnerv1.LogRow{a, b, c}, 0, 1, false, []*runnerv1.LogRow{b, c}, false, false},
		{"all already acked, !NoMore bails", []*runnerv1.LogRow{a, b, c}, 0, 3, false, nil, false, true},

		// Regression coverage for https://gitea.com/gitea/runner/issues/950:
		// a final UpdateLog{NoMore:true} with no new rows must still finalize,
		// otherwise dbfs_data rows leak.
		{"empty rows + NoMore must finalize", nil, 5, 5, true, nil, true, false},
		{"empty rows, !NoMore bails", nil, 5, 5, false, nil, false, true},
		{"already-acked re-send + NoMore must finalize", []*runnerv1.LogRow{a, b, c}, 0, 3, true, nil, true, false},

		// Runner ahead of the server: bail even with NoMore, otherwise we'd
		// archive a log with a gap.
		{"runner ahead of server, !NoMore bails", []*runnerv1.LogRow{a}, 5, 0, false, nil, false, true},
		{"runner ahead of server, NoMore still bails", []*runnerv1.LogRow{a}, 5, 0, true, nil, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRows, gotFinalize, gotBail := planLogUpdate(tc.rows, tc.index, tc.ack, tc.noMore)
			assert.Equal(t, tc.wantNewRows, gotRows, "newRows")
			assert.Equal(t, tc.wantFinalize, gotFinalize, "finalize")
			assert.Equal(t, tc.wantBail, gotBail, "bail")
		})
	}
}
