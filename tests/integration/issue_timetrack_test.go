// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestIssueTimeDeleteScoped(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, issue1.LoadRepo(t.Context()))
	tracked := unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{ID: 5})

	session := loginUser(t, issue1.Repo.OwnerName)
	url := fmt.Sprintf("/%s/%s/issues/%d/times/%d/delete", issue1.Repo.OwnerName, issue1.Repo.Name, issue1.Index, tracked.ID)
	req := NewRequestWithValues(t, "POST", url, map[string]string{})
	session.MakeRequest(t, req, http.StatusNotFound)

	tracked = unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{ID: tracked.ID})
	assert.False(t, tracked.Deleted)
}

func TestIssueTimeAddSpentOn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, issue1.LoadRepo(t.Context()))

	session := loginUser(t, issue1.Repo.OwnerName)
	url := fmt.Sprintf("/%s/%s/issues/%d/times/add", issue1.Repo.OwnerName, issue1.Repo.Name, issue1.Index)
	req := NewRequestWithValues(t, "POST", url, map[string]string{
		"hours":    "2",
		"minutes":  "30",
		"spent_on": "2026-07-08",
	})
	session.MakeRequest(t, req, http.StatusOK)

	created, err := time.ParseInLocation(time.DateOnly, "2026-07-08", setting.DefaultUILocation)
	assert.NoError(t, err)
	tracked := unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{UserID: 2, IssueID: issue1.ID, Time: 9000})
	assert.Equal(t, created.Unix(), tracked.SpentOnUnix)
	assert.NotEqual(t, tracked.CreatedUnix, tracked.SpentOnUnix)
}
