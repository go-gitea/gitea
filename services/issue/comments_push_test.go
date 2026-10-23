// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCommentsPushCommits(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	const knownCommit = "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	newPushComment := func(isForce bool, ids ...string) *issues_model.Comment {
		content, err := json.Marshal(issues_model.PushActionContent{IsForcePush: isForce, CommitIDs: ids})
		require.NoError(t, err)
		return &issues_model.Comment{
			Type:    issues_model.CommentTypePullRequestPush,
			IssueID: issue.ID,
			Issue:   issue,
			Content: string(content),
		}
	}

	normal1 := newPushComment(false, knownCommit, "0000000000000000000000000000000000000001")
	normal2 := newPushComment(false, knownCommit)
	force := newPushComment(true, "aaaa", "bbbb")
	broken := &issues_model.Comment{Type: issues_model.CommentTypePullRequestPush, Content: "{broken"}
	other := &issues_model.Comment{Type: issues_model.CommentTypeComment, Content: "hello"}

	require.NoError(t, LoadCommentsPushCommits(t.Context(), []*issues_model.Comment{normal1, normal2, force, broken, other}))

	// commits shared by several comments are resolved once, unknown commits are skipped
	require.Len(t, normal1.Commits, 1)
	assert.Equal(t, int64(1), normal1.CommitsNum)
	assert.Equal(t, knownCommit, normal1.Commits[0].GitCommit.ID.String())
	require.Len(t, normal2.Commits, 1)
	assert.Same(t, normal1.Commits[0], normal2.Commits[0])

	assert.True(t, force.IsForcePush)
	assert.Equal(t, "aaaa", force.OldCommit)
	assert.Equal(t, "bbbb", force.NewCommit)
	assert.Nil(t, force.Commits)
	assert.Nil(t, broken.Commits)
	assert.Nil(t, other.Commits)

	// the single-comment helper keeps working
	single := newPushComment(false, knownCommit)
	require.NoError(t, LoadCommentPushCommits(t.Context(), single))
	assert.Len(t, single.Commits, 1)
}
