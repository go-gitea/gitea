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

func TestAPIGetTrackedTimes(t *testing.T) {
	defer prepareTestEnv(t)()

	user1 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	issue2 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 2}).(*models.Issue)
	assert.NoError(t, issue2.LoadRepo())

	session := loginUser(t, user1.Name)
	token := getTokenForLoggedInUser(t, session)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/times?token=%s", "user2", issue2.Repo.Name, issue2.Index, token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiTimes api.TrackedTimeList
	DecodeJSON(t, resp, &apiTimes)
	expect, err := models.GetTrackedTimes(models.FindTrackedTimesOptions{IssueID:issue2.ID})
	assert.NoError(t, err)
	assert.Len(t, apiTimes, 3)

	for i, time := range expect {
		assert.Equal(t, time.ID, apiTimes[i].ID)
		assert.EqualValues(t, issue2.APIFormat(), apiTimes[i].Issue)
		assert.Equal(t, time.Created.Unix(), apiTimes[i].Created.Unix())
		assert.Equal(t, time.Time, apiTimes[i].Time)
		user, err := models.GetUserByID(time.UserID)
		assert.NoError(t, err)
		assert.Equal(t, user.Name, apiTimes[i].UserName)
	}
}
