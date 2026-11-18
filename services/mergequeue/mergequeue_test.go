// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"testing"

	git_model "gitea.dev/models/git"
	pull_model "gitea.dev/models/pull"

	"github.com/stretchr/testify/assert"
)

func TestHasPendingOrPassingAfter(t *testing.T) {
	s := &queueState{
		cfg: git_model.MergeQueueConfig{MaxEntriesToMerge: 1},
		entries: []*pull_model.MergeQueueEntry{
			{State: pull_model.MergeQueueStateUnmergeable},
			{State: pull_model.MergeQueueStateMergeable},
		},
	}
	// a passing entry outside the merge group size can't merge the failed one, so it must not keep it
	assert.False(t, s.hasPendingOrPassingAfter(0))
	assert.False(t, s.hasPendingOrPassingAfter(1), "an entry beyond the merge group size must not panic")

	s.cfg.MaxEntriesToMerge = 2
	assert.True(t, s.hasPendingOrPassingAfter(0))
}
