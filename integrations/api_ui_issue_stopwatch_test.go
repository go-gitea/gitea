// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestUIAPIListStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	req := NewRequestf(t, "GET", "/api/ui/user/stopwatches")
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiWatches []*api.StopWatch
	DecodeJSON(t, resp, &apiWatches)
	stopwatch := models.AssertExistsAndLoadBean(t, &models.Stopwatch{UserID: owner.ID}).(*models.Stopwatch)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: stopwatch.IssueID}).(*models.Issue)
	if assert.Len(t, apiWatches, 1) {
		assert.EqualValues(t, stopwatch.CreatedUnix.AsTime().Unix(), apiWatches[0].Created.Unix())
		assert.EqualValues(t, issue.Index, apiWatches[0].IssueIndex)
		assert.EqualValues(t, issue.Title, apiWatches[0].IssueTitle)
		assert.EqualValues(t, repo.Name, apiWatches[0].RepoName)
		assert.EqualValues(t, repo.OwnerName, apiWatches[0].RepoOwnerName)
		assert.Greater(t, int64(apiWatches[0].Seconds), int64(0))
	}
}

func TestAPIUIStopStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 2}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	session := loginUser(t, user.Name)

	req := NewRequestf(t, "POST", "/api/v1/repos/%s/%s/issues/%d/stopwatch/stop", owner.Name, issue.Repo.Name, issue.Index)
	session.MakeRequest(t, req, http.StatusCreated)
	session.MakeRequest(t, req, http.StatusConflict)
}

func TestAPIUICancelStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)

	session := loginUser(t, user.Name)

	req := NewRequestf(t, "DELETE", "/api/ui/repos/%s/%s/issues/%d/stopwatch/delete", owner.Name, issue.Repo.Name, issue.Index)
	session.MakeRequest(t, req, http.StatusNoContent)
	session.MakeRequest(t, req, http.StatusConflict)
}

func TestAPIUIStartStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 3}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	session := loginUser(t, user.Name)

	req := NewRequestf(t, "POST", "/api/v1/repos/%s/%s/issues/%d/stopwatch/start", owner.Name, issue.Repo.Name, issue.Index)
	session.MakeRequest(t, req, http.StatusCreated)
	session.MakeRequest(t, req, http.StatusConflict)
}
