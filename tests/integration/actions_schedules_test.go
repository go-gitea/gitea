// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsSchedules(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()

	// Clean slate
	require.NoError(t, db.DeleteAllRecords("action_schedule"))
	require.NoError(t, db.DeleteAllRecords("action_schedule_spec"))

	adminSession := loginUser(t, "user1")

	// Create test data
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	require.NoError(t, actions_model.CreateScheduleTask(ctx, []*actions_model.ActionSchedule{
		{
			Title:         "ci.yml",
			Specs:         []string{"0 * * * *"},
			RepoID:        repo1.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "w1",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/main",
			CommitSHA:     "abc123",
		},
		{
			Title:         "deploy.yml",
			Specs:         []string{"30 2 * * *"},
			RepoID:        repo1.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "w2",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/main",
			CommitSHA:     "def456",
		},
		{
			Title:         "nightly.yml",
			Specs:         []string{"0 0 * * *", "0 12 * * *"},
			RepoID:        repo2.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "w3",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/develop",
			CommitSHA:     "ghi789",
		},
	}))

	t.Run("Admin", func(t *testing.T) {
		t.Run("PageRenders", func(t *testing.T) {
			req := NewRequest(t, "GET", "/-/admin/actions/schedules")
			resp := adminSession.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			assert.True(t, test.IsNormalPageCompleted(body))
		})

		t.Run("ShowsAllSchedules", func(t *testing.T) {
			req := NewRequest(t, "GET", "/-/admin/actions/schedules")
			resp := adminSession.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			assert.Contains(t, body, "ci.yml")
			assert.Contains(t, body, "deploy.yml")
			assert.Contains(t, body, "nightly.yml")
			assert.Contains(t, body, "0 * * * *")
			assert.Contains(t, body, "30 2 * * *")
			assert.Contains(t, body, "0 0 * * *")
			assert.Contains(t, body, "0 12 * * *")
			assert.Contains(t, body, "refs/heads/main")
			assert.Contains(t, body, "refs/heads/develop")
			// Admin view shows both repos
			assert.Contains(t, body, fmt.Sprintf("/%s/%s", repo1.OwnerName, repo1.Name))
			assert.Contains(t, body, fmt.Sprintf("/%s/%s", repo2.OwnerName, repo2.Name))
		})

		t.Run("NonAdminDenied", func(t *testing.T) {
			sessionUser := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/-/admin/actions/schedules")
			sessionUser.MakeRequest(t, req, http.StatusForbidden)
		})
	})

	t.Run("Repo", func(t *testing.T) {
		repoWebURL := fmt.Sprintf("/%s/%s/settings/actions/schedules", repo1.OwnerName, repo1.Name)
		sessionRepoAdmin := loginUser(t, repo1.OwnerName)

		t.Run("PageRenders", func(t *testing.T) {
			req := NewRequest(t, "GET", repoWebURL)
			resp := sessionRepoAdmin.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			assert.True(t, test.IsNormalPageCompleted(body))
		})

		t.Run("ShowsRepoSchedulesOnly", func(t *testing.T) {
			req := NewRequest(t, "GET", repoWebURL)
			resp := sessionRepoAdmin.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			// repo1 schedules should be present
			assert.Contains(t, body, "ci.yml")
			assert.Contains(t, body, "deploy.yml")
			assert.Contains(t, body, "0 * * * *")
			assert.Contains(t, body, "30 2 * * *")
			assert.Contains(t, body, "refs/heads/main")
			// repo2 schedules should NOT be present
			assert.NotContains(t, body, "nightly.yml")
			assert.NotContains(t, body, "0 0 * * *")
			assert.NotContains(t, body, "0 12 * * *")
			assert.NotContains(t, body, "refs/heads/develop")
		})

		t.Run("RepoOwnerCanAccess", func(t *testing.T) {
			sessionOwner := loginUser(t, user2.Name)
			req := NewRequest(t, "GET", repoWebURL)
			sessionOwner.MakeRequest(t, req, http.StatusOK)
		})

		t.Run("NonAdminDenied", func(t *testing.T) {
			// user4 has no access to repo1, so gets 404 (not 403) to avoid leaking existence
			sessionUser4 := loginUser(t, "user4")
			req := NewRequest(t, "GET", repoWebURL)
			sessionUser4.MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
