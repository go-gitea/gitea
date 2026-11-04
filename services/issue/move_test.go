// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveIssue(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	srcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	targetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	require.False(t, issue.IsClosed)

	require.NoError(t, MoveIssue(t.Context(), doer, issue, targetRepo, MoveIssueOptions{KeepLabels: true}))

	moved := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.Equal(t, targetRepo.ID, moved.RepoID)
	byIndex, err := issues_model.GetIssueByIndex(t.Context(), targetRepo.ID, moved.Index)
	require.NoError(t, err)
	assert.Equal(t, moved.ID, byIndex.ID)

	redirect, err := issues_model.GetIssueRedirect(t.Context(), srcRepo.ID, 1)
	require.NoError(t, err)
	require.NotNil(t, redirect)
	assert.Equal(t, moved.ID, redirect.IssueID)

	// label1 belongs to repo1, so a label with the same name is created in the target repository
	labels, err := issues_model.GetLabelsByIssueID(t.Context(), moved.ID)
	require.NoError(t, err)
	require.Len(t, labels, 1)
	assert.Equal(t, "label1", labels[0].Name)
	assert.Equal(t, targetRepo.ID, labels[0].RepoID)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 1, IssueID: moved.ID, RepoID: targetRepo.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: moved.ID, Type: issues_model.CommentTypeMoveIssueFrom, Content: "user2/repo1#1"})

	newSrcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: srcRepo.ID})
	assert.Equal(t, srcRepo.NumIssues-1, newSrcRepo.NumIssues)
	newTargetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: targetRepo.ID})
	assert.Equal(t, targetRepo.NumIssues+1, newTargetRepo.NumIssues)
}

func TestCopyIssue(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	srcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	targetRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	newIssue, err := CopyIssue(t.Context(), doer, issue, targetRepo, CopyIssueOptions{
		MoveIssueOptions: MoveIssueOptions{KeepLabels: true},
		CopyAssignees:    true,
	})
	require.NoError(t, err)

	// the original issue stays where it was
	unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID, RepoID: srcRepo.ID, Index: issue.Index})

	copied := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: newIssue.ID})
	assert.Equal(t, targetRepo.ID, copied.RepoID)
	assert.Equal(t, issue.Title, copied.Title)
	assert.Equal(t, issue.Content, copied.Content)
	assert.Equal(t, doer.ID, copied.PosterID)

	labels, err := issues_model.GetLabelsByIssueID(t.Context(), copied.ID)
	require.NoError(t, err)
	require.Len(t, labels, 1)
	assert.Equal(t, "label1", labels[0].Name)
	assert.Equal(t, targetRepo.ID, labels[0].RepoID)

	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueAssignees{IssueID: copied.ID, AssigneeID: 1})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeCopyIssueTo, RefRepoID: targetRepo.ID})

	// users who can't read the private target repository don't see where the issue was copied to
	comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{IssueID: issue.ID, Type: issues_model.CommentTypeCopyIssueTo})
	require.NoError(t, err)
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	perm5, err := access_model.GetDoerRepoPermission(t.Context(), srcRepo, user5)
	require.NoError(t, err)
	readable, err := FilterReadableComments(t.Context(), user5, perm5, issue, comments)
	require.NoError(t, err)
	assert.Empty(t, readable)
	perm2, err := access_model.GetDoerRepoPermission(t.Context(), srcRepo, doer)
	require.NoError(t, err)
	readable, err = FilterReadableComments(t.Context(), doer, perm2, issue, comments)
	require.NoError(t, err)
	assert.Len(t, readable, 1)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: copied.ID, Type: issues_model.CommentTypeCopyIssueFrom, Content: "user2/repo1#1"})
}

func TestMoveIssueInvalidTarget(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})

	err := MoveIssue(t.Context(), doer, issue, repo1, MoveIssueOptions{})
	assert.ErrorIs(t, err, util.ErrInvalidArgument)

	err = MoveIssue(t.Context(), doer, pull, repo2, MoveIssueOptions{})
	assert.ErrorIs(t, err, util.ErrInvalidArgument)

	// dependencies are not enabled in the fixture repositories
	defer test.MockVariableValue(&setting.Service.AllowCrossRepositoryDependencies, true)()
	_, err = CopyIssue(t.Context(), doer, issue, repo2, CopyIssueOptions{Dependency: CopyIssueDependencyOriginalBlocksCopy})
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
	_, err = CopyIssue(t.Context(), doer, issue, repo2, CopyIssueOptions{Dependency: "unknown"})
	assert.ErrorIs(t, err, util.ErrInvalidArgument)

	// user5 can't write issues in the private repo2
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	_, err = CopyIssue(t.Context(), user5, issue, repo2, CopyIssueOptions{})
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
}
