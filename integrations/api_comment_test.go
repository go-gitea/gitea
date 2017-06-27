// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPIListComments(t *testing.T) {
	prepareTestEnv(t)

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{},
		models.Cond("type = ?", models.CommentTypeComment)).(*models.Comment)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/comments",
		repoOwner.Name, repo.Name, issue.Index)
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	var comments []*api.Comment
	DecodeJSON(t, resp, &comments)
	expectedCount := models.GetCount(t, &models.Comment{IssueID: issue.ID},
		models.Cond("type = ?", models.CommentTypeComment))
	assert.EqualValues(t, expectedCount, len(comments))
}
