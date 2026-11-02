// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterReadableCommentsChecksUnitOfSameRepoTargets(t *testing.T) {
	var perm access_model.Permission
	perm.SetUnitsWithDefaultAccessMode([]*repo_model.RepoUnit{{Type: unit.TypeIssues}}, perm_model.AccessModeRead)
	issueRef := &issues_model.Comment{Type: issues_model.CommentTypeIssueRef, RefRepoID: 1}
	pullRef := &issues_model.Comment{Type: issues_model.CommentTypePullRef, RefRepoID: 1, RefIsPull: true}
	issueDependency := &issues_model.Comment{Type: issues_model.CommentTypeAddDependency, DependentIssue: &issues_model.Issue{RepoID: 1}}
	pullDependency := &issues_model.Comment{Type: issues_model.CommentTypeAddDependency, DependentIssue: &issues_model.Issue{RepoID: 1, IsPull: true}}
	repo := &repo_model.Repository{ID: 1}

	comments, err := FilterReadableComments(t.Context(), nil, perm, &issues_model.Issue{RepoID: 1, Repo: repo}, issues_model.CommentList{issueRef, pullRef, issueDependency, pullDependency})
	require.NoError(t, err)
	assert.Equal(t, issues_model.CommentList{issueRef, issueDependency}, comments)
	assert.Same(t, repo, issueRef.RefRepo)
	assert.Same(t, repo, issueDependency.RefRepo)
	assert.Same(t, repo, issueDependency.DependentIssue.Repo)
}
