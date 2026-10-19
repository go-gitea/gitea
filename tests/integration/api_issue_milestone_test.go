// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssuesMilestone(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: milestone.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	assert.Equal(t, int64(1), int64(milestone.NumIssues))
	assert.Equal(t, structs.StateOpen, milestone.State())

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// update values of issue
	milestoneState := "closed"

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/milestones/%d", owner.Name, repo.Name, milestone.ID)
	req := NewRequestWithJSON(t, "PATCH", urlStr, structs.EditMilestoneOption{
		State: &milestoneState,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiMilestone := DecodeJSON(t, resp, &structs.Milestone{})
	assert.EqualValues(t, "closed", apiMilestone.State)
	// the due date is kept when the request doesn't contain one
	if assert.NotNil(t, apiMilestone.Deadline) {
		assert.Equal(t, milestone.DeadlineUnix.AsTime().Unix(), apiMilestone.Deadline.Unix())
	}

	req = NewRequest(t, "GET", urlStr).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiMilestone2 := DecodeJSON(t, resp, &structs.Milestone{})
	assert.EqualValues(t, "closed", apiMilestone2.State)
	assert.NotNil(t, apiMilestone2.Deadline)

	// the Unix epoch removes the due date
	req = NewRequestWithJSON(t, "PATCH", urlStr, structs.EditMilestoneOption{
		Deadline: new(time.Unix(0, 0).UTC()),
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiMilestone = DecodeJSON(t, resp, &structs.Milestone{})
	assert.Nil(t, apiMilestone.Deadline)
	assert.Zero(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: milestone.ID}).DeadlineUnix)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/milestones", owner.Name, repo.Name), structs.CreateMilestoneOption{
		Title:       "wow",
		Description: "closed one",
		State:       "closed",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiMilestone = DecodeJSON(t, resp, &structs.Milestone{})
	assert.Equal(t, "wow", apiMilestone.Title)
	assert.Equal(t, structs.StateClosed, apiMilestone.State)
	assert.Nil(t, apiMilestone.Deadline)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/milestones?state=%s", owner.Name, repo.Name, "all")).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiMilestones := DecodeJSON(t, resp, []structs.Milestone{})
	assert.Len(t, apiMilestones, 4)
	assert.Nil(t, apiMilestones[0].Deadline)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/milestones/%s", owner.Name, repo.Name, apiMilestones[2].Title)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiMilestone = DecodeJSON(t, resp, &structs.Milestone{})
	assert.Equal(t, apiMilestones[2], *apiMilestone)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/milestones?state=%s&name=%s", owner.Name, repo.Name, "all", "milestone2")).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiMilestones = DecodeJSON(t, resp, []structs.Milestone{})
	assert.Len(t, apiMilestones, 1)
	assert.Equal(t, int64(2), apiMilestones[0].ID)

	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/milestones/%d", owner.Name, repo.Name, apiMilestone.ID)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}
