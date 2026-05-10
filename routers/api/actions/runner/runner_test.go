// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestPlanLogUpdate(t *testing.T) {
	a := &runnerv1.LogRow{Content: "a"}
	abc := []*runnerv1.LogRow{a, a, a}

	cases := []struct {
		name       string
		rows       []*runnerv1.LogRow
		index, ack int64
		noMore     bool
		wantRows   []*runnerv1.LogRow
		wantBail   bool
	}{
		// Regression for https://gitea.com/gitea/runner/issues/950: empty
		// rows + NoMore must fall through so TransferLogs runs.
		{"empty + NoMore must not bail", nil, 5, 5, true, nil, false},
		{"empty + !NoMore bails", nil, 5, 5, false, nil, true},
		{"trims acked prefix", abc, 0, 1, false, []*runnerv1.LogRow{a, a}, false},
		{"runner ahead bails even with NoMore", abc, 5, 0, true, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, bail := planLogUpdate(tc.rows, tc.index, tc.ack, tc.noMore)
			assert.Equal(t, tc.wantRows, rows)
			assert.Equal(t, tc.wantBail, bail)
		})
	}
}
