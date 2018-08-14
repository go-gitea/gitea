// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPIViewPulls(t *testing.T) {
	prepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all", owner.Name, repo.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var pulls []*api.PullRequest
	DecodeJSON(t, resp, &pulls)
	expectedLen := models.GetCount(t, &models.Issue{RepoID: repo.ID}, models.Cond("is_pull = ?", true))
	assert.Len(t, pulls, expectedLen)
}

// TestAPIMergePullWIP ensures that we can't merge a WIP pull request
func TestAPIMergePullWIP(t *testing.T) {
	prepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{Status: models.PullRequestStatusMergeable}, models.Cond("has_merged = ?", false)).(*models.PullRequest)
	pr.LoadIssue()
	pr.Issue.ChangeTitle(owner, setting.Repository.PullRequest.WorkInProgressPrefixes[0]+" "+pr.Issue.Title)

	// force reload
	pr.LoadAttributes()

	assert.Contains(t, pr.Issue.Title, setting.Repository.PullRequest.WorkInProgressPrefixes[0])

	session := loginUser(t, owner.Name)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner.Name, repo.Name, pr.Index), &auth.MergePullRequestForm{
		MergeMessageField: pr.Issue.Title,
		Do:                string(models.MergeStyleMerge),
	})

	session.MakeRequest(t, req, http.StatusMethodNotAllowed)
}
