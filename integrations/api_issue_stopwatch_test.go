// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIListStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/user/stopwatches?token=%s", token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiWatches []*api.StopWatch
	DecodeJSON(t, resp, &apiWatches)
	stopwatch := db.AssertExistsAndLoadBean(t, &models.Stopwatch{UserID: owner.ID}).(*models.Stopwatch)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: stopwatch.IssueID}).(*models.Issue)
	if assert.Len(t, apiWatches, 1) {
		assert.EqualValues(t, stopwatch.CreatedUnix.AsTime().Unix(), apiWatches[0].Created.Unix())
		assert.EqualValues(t, issue.Index, apiWatches[0].IssueIndex)
		assert.EqualValues(t, issue.Title, apiWatches[0].IssueTitle)
		assert.EqualValues(t, repo.Name, apiWatches[0].RepoName)
		assert.EqualValues(t, repo.OwnerName, apiWatches[0].RepoOwnerName)
		assert.Greater(t, int64(apiWatches[0].Seconds), int64(0))
	}
}

func TestAPIStopStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: 2}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := db.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)
	user := db.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	req := NewRequestf(t, "POST", "/api/v1/repos/%s/%s/issues/%d/stopwatch/stop?token=%s", owner.Name, issue.Repo.Name, issue.Index, token)
	session.MakeRequest(t, req, http.StatusCreated)
	session.MakeRequest(t, req, http.StatusConflict)
}

func TestAPICancelStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: 1}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := db.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)
	user := db.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/%d/stopwatch/delete?token=%s", owner.Name, issue.Repo.Name, issue.Index, token)
	session.MakeRequest(t, req, http.StatusNoContent)
	session.MakeRequest(t, req, http.StatusConflict)
}

func TestAPIStartStopWatches(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: 3}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := db.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)
	user := db.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	req := NewRequestf(t, "POST", "/api/v1/repos/%s/%s/issues/%d/stopwatch/start?token=%s", owner.Name, issue.Repo.Name, issue.Index, token)
	session.MakeRequest(t, req, http.StatusCreated)
	session.MakeRequest(t, req, http.StatusConflict)
}
