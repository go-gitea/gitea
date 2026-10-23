// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	git_model "gitea.dev/models/git"

	"github.com/stretchr/testify/assert"
)

func TestGetCommitActionsStatusMapCached(t *testing.T) {
	// no database in this package's tests: any query would fail the test by returning nil/panicking,
	// so a covered lookup must be served from the preloaded cache only
	data := map[string]any{}
	data[commitActionsStatusCacheKey] = &CommitActionsStatusCache{
		covered: map[int64]struct{}{1: {}, 2: {}},
		info:    CommitActionsStatusMap{1: actions_model.StatusRunning},
	}

	s1, s2 := &git_model.CommitStatus{ID: 1}, &git_model.CommitStatus{ID: 2}
	got := GetCommitActionsStatusMapCached(t.Context(), data, []*git_model.CommitStatus{s1, nil, s2})
	assert.Equal(t, actions_model.StatusRunning.String(), got.IconStatus(s1))
	assert.Empty(t, got.IconStatus(s2)) // covered, but not backed by Actions

	// statuses that are not covered fall back to the regular resolution (nothing to resolve without a target URL)
	assert.Empty(t, GetCommitActionsStatusMapCached(t.Context(), data, []*git_model.CommitStatus{{ID: 3}}))
	assert.Empty(t, GetCommitActionsStatusMapCached(t.Context(), map[string]any{}, []*git_model.CommitStatus{s1}))
}

func TestPreloadCommitActionsStatuses(t *testing.T) {
	data := map[string]any{}
	PreloadCommitActionsStatuses(t.Context(), data, nil)
	assert.Empty(t, data)

	// statuses without a target URL need no query, but must still be recorded as covered
	PreloadCommitActionsStatuses(t.Context(), data, []*git_model.CommitStatus{{ID: 7}})
	cache, ok := data[commitActionsStatusCacheKey].(*CommitActionsStatusCache)
	assert.True(t, ok)
	assert.Contains(t, cache.covered, int64(7))
}
