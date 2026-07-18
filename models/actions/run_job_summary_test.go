// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActionRunJobSummariesVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const repoID, runID, attemptID int64 = 4, 9601, 0
	const jobA, jobB int64 = 71, 72

	version := func(jobID int64) string {
		v, err := GetActionRunJobSummariesVersion(ctx, repoID, runID, attemptID, jobID)
		require.NoError(t, err)
		return v
	}

	// No summaries yet: empty fingerprint.
	assert.Empty(t, version(0))

	// First summary for job A: fingerprint becomes non-empty and stable across repeated calls.
	require.NoError(t, UpsertActionRunJobSummary(ctx, repoID, runID, attemptID, jobA, 0, JobSummaryContentTypeMarkdown, []byte("# hello")))
	v1 := version(0)
	assert.NotEmpty(t, v1)
	assert.Equal(t, v1, version(0))

	// A new step (higher row count) changes the fingerprint.
	require.NoError(t, UpsertActionRunJobSummary(ctx, repoID, runID, attemptID, jobA, 1, JobSummaryContentTypeMarkdown, []byte("more")))
	v2 := version(0)
	assert.NotEqual(t, v1, v2)

	// Editing an existing step to a different length changes the fingerprint (content size differs).
	require.NoError(t, UpsertActionRunJobSummary(ctx, repoID, runID, attemptID, jobA, 1, JobSummaryContentTypeMarkdown, []byte("much longer replacement content")))
	v3 := version(0)
	assert.NotEqual(t, v2, v3)

	// Job scoping: adding job B changes the all-jobs fingerprint but leaves job A's untouched.
	vA := version(jobA)
	require.NoError(t, UpsertActionRunJobSummary(ctx, repoID, runID, attemptID, jobB, 0, JobSummaryContentTypeMarkdown, []byte("b summary")))
	assert.NotEqual(t, v3, version(0))
	assert.Equal(t, vA, version(jobA))

	// Deleting a step changes the fingerprint.
	beforeDelete := version(jobA)
	require.NoError(t, DeleteActionRunJobSummary(ctx, repoID, runID, attemptID, jobA, 1))
	assert.NotEqual(t, beforeDelete, version(jobA))
}
