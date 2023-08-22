// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetTrackedTimes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	assert.NoError(t, issue2.LoadRepo(db.DefaultContext))

	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/times?token=%s", user2.Name, issue2.Repo.Name, issue2.Index, token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiTimes api.TrackedTimeList
	DecodeJSON(t, resp, &apiTimes)
	expect, err := issues_model.GetTrackedTimes(db.DefaultContext, &issues_model.FindTrackedTimesOptions{IssueID: issue2.ID})
	assert.NoError(t, err)
	assert.Len(t, apiTimes, 3)

	for i, time := range expect {
		assert.Equal(t, time.ID, apiTimes[i].ID)
		assert.EqualValues(t, issue2.Title, apiTimes[i].Issue.Title)
		assert.EqualValues(t, issue2.ID, apiTimes[i].IssueID)
		assert.Equal(t, time.Created.Unix(), apiTimes[i].Created.Unix())
		assert.Equal(t, time.Time, apiTimes[i].Time)
		user, err := user_model.GetUserByID(db.DefaultContext, time.UserID)
		assert.NoError(t, err)
		assert.Equal(t, user.Name, apiTimes[i].UserName)
	}

	// test filter
	since := "2000-01-01T00%3A00%3A02%2B00%3A00"  // 946684802
	before := "2000-01-01T00%3A00%3A12%2B00%3A00" // 946684812

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/times?since=%s&before=%s&token=%s", user2.Name, issue2.Repo.Name, issue2.Index, since, before, token)
	resp = MakeRequest(t, req, http.StatusOK)
	var filterAPITimes api.TrackedTimeList
	DecodeJSON(t, resp, &filterAPITimes)
	assert.Len(t, filterAPITimes, 2)
	assert.Equal(t, int64(3), filterAPITimes[0].ID)
	assert.Equal(t, int64(6), filterAPITimes[1].ID)
}

func TestAPIDeleteTrackedTime(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	time6 := unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{ID: 6})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	assert.NoError(t, issue2.LoadRepo(db.DefaultContext))
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Deletion not allowed
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/%d/times/%d?token=%s", user2.Name, issue2.Repo.Name, issue2.Index, time6.ID, token)
	MakeRequest(t, req, http.StatusForbidden)

	time3 := unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{ID: 3})
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/%d/times/%d?token=%s", user2.Name, issue2.Repo.Name, issue2.Index, time3.ID, token)
	MakeRequest(t, req, http.StatusNoContent)
	// Delete non existing time
	MakeRequest(t, req, http.StatusNotFound)

	// Reset time of user 2 on issue 2
	trackedSeconds, err := issues_model.GetTrackedSeconds(db.DefaultContext, issues_model.FindTrackedTimesOptions{IssueID: 2, UserID: 2})
	assert.NoError(t, err)
	assert.Equal(t, int64(3661), trackedSeconds)

	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/%d/times?token=%s", user2.Name, issue2.Repo.Name, issue2.Index, token)
	MakeRequest(t, req, http.StatusNoContent)
	MakeRequest(t, req, http.StatusNotFound)

	trackedSeconds, err = issues_model.GetTrackedSeconds(db.DefaultContext, issues_model.FindTrackedTimesOptions{IssueID: 2, UserID: 2})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), trackedSeconds)
}

func TestAPIAddTrackedTimes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	assert.NoError(t, issue2.LoadRepo(db.DefaultContext))
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	session := loginUser(t, admin.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/times?token=%s", user2.Name, issue2.Repo.Name, issue2.Index, token)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.AddTimeOption{
		Time:    33,
		User:    user2.Name,
		Created: time.Unix(947688818, 0),
	})
	resp := MakeRequest(t, req, http.StatusOK)
	var apiNewTime api.TrackedTime
	DecodeJSON(t, resp, &apiNewTime)

	assert.EqualValues(t, 33, apiNewTime.Time)
	assert.EqualValues(t, user2.ID, apiNewTime.UserID)
	assert.EqualValues(t, 947688818, apiNewTime.Created.Unix())
}
