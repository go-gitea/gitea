// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCanSelfChangeOrAccessContent(t *testing.T) {
	doer := &user_model.User{ID: 1}
	for _, u := range []struct {
		name     string
		unitType unit.Type
	}{
		{"issues", unit.TypeIssues},
		{"pulls", unit.TypePullRequests},
	} {
		t.Run(u.name, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				doer         *user_model.User
				posterID     int64
				mode         perm.AccessMode
				requiredMode perm.AccessMode
				locked       bool
				allowed      bool
			}{
				{"anonymous", nil, 1, perm.AccessModeWrite, perm.AccessModeWrite, false, false},
				{"author without read access", doer, 1, perm.AccessModeNone, perm.AccessModeWrite, false, false},
				{"author with read access", doer, 1, perm.AccessModeRead, perm.AccessModeWrite, false, true},
				{"reader cannot edit others", doer, 2, perm.AccessModeRead, perm.AccessModeWrite, false, false},
				{"reader can access others", doer, 2, perm.AccessModeRead, perm.AccessModeRead, false, true},
				{"writer can edit others", doer, 2, perm.AccessModeWrite, perm.AccessModeWrite, false, true},
				{"locked author needs write access", doer, 1, perm.AccessModeRead, perm.AccessModeWrite, true, false},
				{"locked reader needs write access", doer, 2, perm.AccessModeRead, perm.AccessModeRead, true, false},
				{"locked writer can edit", doer, 2, perm.AccessModeWrite, perm.AccessModeWrite, true, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var permission access_model.Permission
					permission.SetUnitsWithDefaultAccessMode([]*repo_model.RepoUnit{{Type: u.unitType}}, tc.mode)
					issue := &issues_model.Issue{IsPull: u.unitType == unit.TypePullRequests, IsLocked: tc.locked}
					assert.Equal(t, tc.allowed, CanSelfChangeOrAccessContent(tc.doer, permission, issue, tc.posterID, tc.requiredMode))
				})
			}
		})
	}
}

func TestGetRefEndNamesAndURLs(t *testing.T) {
	issues := []*issues_model.Issue{
		{ID: 1, Ref: "refs/heads/branch1"},
		{ID: 2, Ref: "refs/tags/tag1"},
		{ID: 3, Ref: "c0ffee"},
	}
	repoLink := "/foo/bar"

	endNames, urls := GetRefEndNamesAndURLs(issues, repoLink)
	assert.Equal(t, map[int64]string{1: "branch1", 2: "tag1", 3: "c0ffee"}, endNames)
	assert.Equal(t, map[int64]string{
		1: repoLink + "/src/branch/branch1",
		2: repoLink + "/src/tag/tag1",
		3: repoLink + "/src/commit/c0ffee",
	}, urls)
}

func TestIssue_DeleteIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueIDs, err := issues_model.GetIssueIDsByRepoID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, issueIDs, 5)

	issue := &issues_model.Issue{
		RepoID: 1,
		ID:     issueIDs[2],
	}

	_, err = deleteIssue(t.Context(), issue)
	assert.NoError(t, err)
	issueIDs, err = issues_model.GetIssueIDsByRepoID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, issueIDs, 4)

	// check attachment removal
	attachments, err := repo_model.GetAttachmentsByIssueID(t.Context(), 4)
	assert.NoError(t, err)
	issue, err = issues_model.GetIssueByID(t.Context(), 4)
	assert.NoError(t, err)
	_, err = deleteIssue(t.Context(), issue)
	assert.NoError(t, err)
	assert.Len(t, attachments, 2)
	for i := range attachments {
		attachment, err := repo_model.GetAttachmentByUUID(t.Context(), attachments[i].UUID)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrAttachmentNotExist(err))
		assert.Nil(t, attachment)
	}

	// check issue dependencies
	user, err := user_model.GetUserByID(t.Context(), 1)
	assert.NoError(t, err)
	issue1, err := issues_model.GetIssueByID(t.Context(), 1)
	assert.NoError(t, err)
	issue2, err := issues_model.GetIssueByID(t.Context(), 2)
	assert.NoError(t, err)
	err = issues_model.CreateIssueDependency(t.Context(), user, issue1, issue2)
	assert.NoError(t, err)
	left, err := issues_model.IssueNoDependenciesLeft(t.Context(), issue1)
	assert.NoError(t, err)
	assert.False(t, left)

	_, err = deleteIssue(t.Context(), issue2)
	assert.NoError(t, err)
	left, err = issues_model.IssueNoDependenciesLeft(t.Context(), issue1)
	assert.NoError(t, err)
	assert.True(t, left)
}
