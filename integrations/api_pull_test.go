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
