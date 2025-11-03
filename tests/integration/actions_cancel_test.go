// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"github.com/stretchr/testify/assert"
)

// This verifies that cancelling a run without running jobs (stuck in waiting) is updated to cancelled status
func TestActionsCancelStuckWaitingRun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		session := loginUser(t, user5.Name)

		// cancel the run by run index
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/cancel", user5.Name, "repo4", 191), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})

		session.MakeRequest(t, req, http.StatusOK)

		// check if the run is cancelled by id
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			ID: 805,
		})
		assert.Equal(t, actions_model.StatusCancelled, run.Status)
	})
}
