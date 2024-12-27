// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

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
	var apiMilestone structs.Milestone
	DecodeJSON(t, resp, &apiMilestone)
	assert.EqualValues(t, "closed", apiMilestone.State)

	req = NewRequest(t, "GET", urlStr).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var apiMilestone2 structs.Milestone
	DecodeJSON(t, resp, &apiMilestone2)
	assert.EqualValues(t, "closed", apiMilestone2.State)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/milestones", owner.Name, repo.Name), structs.CreateMilestoneOption{
		Title:       "wow",
		Description: "closed one",
		State:       "closed",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiMilestone)
	assert.Equal(t, "wow", apiMilestone.Title)
	assert.Equal(t, structs.StateClosed, apiMilestone.State)
	assert.Nil(t, apiMilestone.Deadline)

	var apiMilestones []structs.Milestone
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/milestones?state=%s", owner.Name, repo.Name, "all")).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiMilestones)
	assert.Len(t, apiMilestones, 4)
	assert.Nil(t, apiMilestones[0].Deadline)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/milestones/%s", owner.Name, repo.Name, apiMilestones[2].Title)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiMilestone)
	assert.EqualValues(t, apiMilestones[2], apiMilestone)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/milestones?state=%s&name=%s", owner.Name, repo.Name, "all", "milestone2")).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiMilestones)
	assert.Len(t, apiMilestones, 1)
	assert.Equal(t, int64(2), apiMilestones[0].ID)

	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/milestones/%d", owner.Name, repo.Name, apiMilestone.ID)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}
