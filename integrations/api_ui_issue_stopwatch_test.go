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
