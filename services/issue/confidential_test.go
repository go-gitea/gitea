// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"testing"

	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type confidentialRecorder struct {
	notify_service.NullNotifier
	calls []bool // the persisted flag at the moment the notifier ran; true proves the transaction had committed
}

var confidentialChanges = &confidentialRecorder{}

func (r *confidentialRecorder) IssueChangeConfidential(ctx context.Context, _ *user_model.User, issue *issues_model.Issue) {
	stored, err := issues_model.GetIssueByID(ctx, issue.ID)
	if err != nil {
		panic(err)
	}
	r.calls = append(r.calls, stored.IsConfidential)
}

func init() {
	notify_service.RegisterNotifier(confidentialChanges)
}

func permissionWithAccess(mode perm_model.AccessMode) access_model.Permission {
	var perm access_model.Permission
	perm.SetUnitsWithDefaultAccessMode([]*repo_model.RepoUnit{{Type: unit.TypeIssues}}, mode)
	return perm
}

func TestChangeConfidential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	writer := permissionWithAccess(perm_model.AccessModeWrite)
	reader := permissionWithAccess(perm_model.AccessModeRead)

	newIssue := func() *issues_model.Issue {
		return unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}, "is_pull = false")
	}
	commentCount := func(issue *issues_model.Issue, commentType issues_model.CommentType) int {
		return unittest.GetCount(t, &issues_model.Comment{IssueID: issue.ID, Type: commentType})
	}

	t.Run("writer toggles and the indexer-facing notifier fires once after commit", func(t *testing.T) {
		*confidentialChanges = confidentialRecorder{}
		issue := newIssue()
		require.NoError(t, ChangeConfidential(t.Context(), issue, doer, writer, true))
		assert.Equal(t, []bool{true}, confidentialChanges.calls)
		assert.Equal(t, 1, commentCount(issue, issues_model.CommentTypeMakeConfidential))

		require.NoError(t, ChangeConfidential(t.Context(), issue, doer, writer, false))
		assert.Equal(t, []bool{true, false}, confidentialChanges.calls)
		assert.Equal(t, 1, commentCount(issue, issues_model.CommentTypeMakePublic))
	})

	t.Run("non-writer is denied", func(t *testing.T) {
		*confidentialChanges = confidentialRecorder{}
		issue := newIssue()
		err := ChangeConfidential(t.Context(), issue, doer, reader, true)
		require.ErrorIs(t, err, ErrConfidentialPermissionDenied)
		assert.ErrorIs(t, err, util.ErrPermissionDenied)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
		assert.Empty(t, confidentialChanges.calls)
	})

	t.Run("same value is a no-op for any role", func(t *testing.T) {
		*confidentialChanges = confidentialRecorder{}
		issue := newIssue()
		before := commentCount(issue, issues_model.CommentTypeMakePublic)
		require.NoError(t, ChangeConfidential(t.Context(), issue, doer, reader, false))
		assert.Empty(t, confidentialChanges.calls)
		assert.Equal(t, before, commentCount(issue, issues_model.CommentTypeMakePublic))
	})

	t.Run("pull requests", func(t *testing.T) {
		*confidentialChanges = confidentialRecorder{}
		pull := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}, "is_pull = true")
		require.ErrorIs(t, ChangeConfidential(t.Context(), pull, doer, writer, true), ErrConfidentialPullRequest)
		require.NoError(t, ChangeConfidential(t.Context(), pull, doer, writer, false))
		assert.Empty(t, confidentialChanges.calls)
	})
}
