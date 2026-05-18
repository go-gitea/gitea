// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpsertAnalysis exercises the insert, update, tag-set replacement, and
// cross-repo tag filtering paths of UpsertAnalysis, plus deletion cascade.
func TestUpsertAnalysis(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const repoID = int64(1)
	const otherRepoID = int64(2)
	const runID = int64(1)
	const attemptID = int64(1)
	const authorID = int64(1)

	// Seed: two tags on repoID, one tag on otherRepoID.
	tagA := &ActionRunFailureTag{RepoID: repoID, Name: "flaky-test", Color: "#ff0000"}
	tagB := &ActionRunFailureTag{RepoID: repoID, Name: "infra", Color: "#00ff00"}
	tagOther := &ActionRunFailureTag{RepoID: otherRepoID, Name: "elsewhere"}
	require.NoError(t, CreateFailureTag(ctx, tagA))
	require.NoError(t, CreateFailureTag(ctx, tagB))
	require.NoError(t, CreateFailureTag(ctx, tagOther))

	// Insert path: create analysis with one tag.
	a1, err := UpsertAnalysis(ctx, repoID, runID, attemptID, authorID, "first note", []int64{tagA.ID})
	require.NoError(t, err)
	assert.NotZero(t, a1.ID)
	assert.Equal(t, "first note", a1.Note)
	tags, err := GetAnalysisTags(ctx, a1.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, tagA.ID, tags[0].ID)

	// Update path: same attempt, replace tag set + note. Other-repo tag must be filtered out.
	a2, err := UpsertAnalysis(ctx, repoID, runID, attemptID, authorID, "second note", []int64{tagB.ID, tagOther.ID})
	require.NoError(t, err)
	assert.Equal(t, a1.ID, a2.ID, "upsert must reuse the existing row, not insert a new one")
	assert.Equal(t, "second note", a2.Note)
	tags, err = GetAnalysisTags(ctx, a2.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1, "tag from a different repo must be silently dropped")
	assert.Equal(t, tagB.ID, tags[0].ID)

	// Delete path: tag links must be gone, row must be gone.
	require.NoError(t, DeleteAnalysis(ctx, repoID, attemptID))
	_, err = GetAnalysisByAttemptID(ctx, attemptID)
	assert.ErrorIs(t, err, util.ErrNotExist)
	count, err := db.GetEngine(ctx).Where("analysis_id = ?", a2.ID).Count(new(ActionRunAnalysisTag))
	require.NoError(t, err)
	assert.Zero(t, count, "delete must cascade to analysis_tag rows")
}

// TestDeleteFailureTag verifies that removing a tag also removes its links from analyses.
func TestDeleteFailureTag(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const repoID = int64(1)
	tag := &ActionRunFailureTag{RepoID: repoID, Name: "to-be-deleted"}
	require.NoError(t, CreateFailureTag(ctx, tag))

	a, err := UpsertAnalysis(ctx, repoID, 2, 2, 1, "n", []int64{tag.ID})
	require.NoError(t, err)
	tags, err := GetAnalysisTags(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)

	require.NoError(t, DeleteFailureTag(ctx, repoID, tag.ID))

	tags, err = GetAnalysisTags(ctx, a.ID)
	require.NoError(t, err)
	assert.Empty(t, tags, "deleting a tag must drop its analysis links")
}
