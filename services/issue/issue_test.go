// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetRefEndNamesAndURLs(t *testing.T) {
	issues := []*issues_model.Issue{
		{ID: 1, Ref: "refs/heads/branch1"},
		{ID: 2, Ref: "refs/tags/tag1"},
		{ID: 3, Ref: "c0ffee"},
	}
	repoLink := "/foo/bar"

	endNames, urls := GetRefEndNamesAndURLs(issues, repoLink)
	assert.EqualValues(t, map[int64]string{1: "branch1", 2: "tag1", 3: "c0ffee"}, endNames)
	assert.EqualValues(t, map[int64]string{
		1: repoLink + "/src/branch/branch1",
		2: repoLink + "/src/tag/tag1",
		3: repoLink + "/src/commit/c0ffee",
	}, urls)
}

func TestIssue_DeleteIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueIDs, err := issues_model.GetIssueIDsByRepoID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Len(t, issueIDs, 5)

	issue := &issues_model.Issue{
		RepoID: 1,
		ID:     issueIDs[2],
	}

	err = deleteIssue(db.DefaultContext, issue)
	assert.NoError(t, err)
	issueIDs, err = issues_model.GetIssueIDsByRepoID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Len(t, issueIDs, 4)

	// check attachment removal
	attachments, err := repo_model.GetAttachmentsByIssueID(db.DefaultContext, 4)
	assert.NoError(t, err)
	issue, err = issues_model.GetIssueByID(db.DefaultContext, 4)
	assert.NoError(t, err)
	err = deleteIssue(db.DefaultContext, issue)
	assert.NoError(t, err)
	assert.Len(t, attachments, 2)
	for i := range attachments {
		attachment, err := repo_model.GetAttachmentByUUID(db.DefaultContext, attachments[i].UUID)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrAttachmentNotExist(err))
		assert.Nil(t, attachment)
	}

	// check issue dependencies
	user, err := user_model.GetUserByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	issue1, err := issues_model.GetIssueByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	issue2, err := issues_model.GetIssueByID(db.DefaultContext, 2)
	assert.NoError(t, err)
	err = issues_model.CreateIssueDependency(user, issue1, issue2)
	assert.NoError(t, err)
	left, err := issues_model.IssueNoDependenciesLeft(db.DefaultContext, issue1)
	assert.NoError(t, err)
	assert.False(t, left)

	err = deleteIssue(db.DefaultContext, issue2)
	assert.NoError(t, err)
	left, err = issues_model.IssueNoDependenciesLeft(db.DefaultContext, issue1)
	assert.NoError(t, err)
	assert.True(t, left)
}
