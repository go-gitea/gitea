// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssuesMilestone(t *testing.T) {
	defer prepareTestEnv(t)()

	milestone := models.AssertExistsAndLoadBean(t, &models.Milestone{ID: 1}).(*models.Milestone)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: milestone.RepoID}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	assert.Equal(t, int64(1), int64(milestone.NumIssues))
	assert.Equal(t, structs.StateOpen, milestone.State())

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	// update values of issue
	milestoneState := "closed"

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/milestones/%d?token=%s", owner.Name, repo.Name, milestone.ID, token)
	req := NewRequestWithJSON(t, "PATCH", urlStr, structs.EditMilestoneOption{
		State: &milestoneState,
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiMilestone structs.Milestone
	DecodeJSON(t, resp, &apiMilestone)
	assert.EqualValues(t, "closed", apiMilestone.State)

	req = NewRequest(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiMilestone2 structs.Milestone
	DecodeJSON(t, resp, &apiMilestone2)
	assert.EqualValues(t, "closed", apiMilestone2.State)
}
