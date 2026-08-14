// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	actions_model "gitea.dev/models/actions"
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

	adminSession := loginUser(t, "user1")

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	require.NoError(t, actions_model.CreateScheduleTask(ctx, []*actions_model.ActionSchedule{
		{
			Title:         "add ci workflow",
			Specs:         []string{"0 * * * *"},
			RepoID:        repo1.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "ci.yml",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/main",
			CommitSHA:     "abc123",
		},
		{
			Title:         "add deploy workflow",
			Specs:         []string{"30 2 * * *", "totally-not-a-cron-expression"},
			RepoID:        repo1.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "deploy.yml",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/main",
			CommitSHA:     "def456",
		},
		{
			Title:         "add nightly workflow",
			Specs:         []string{"0 0 * * *", "0 12 * * *"},
			RepoID:        repo2.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "nightly.yml",
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
			assert.NotContains(t, body, "add ci workflow")
			assert.Contains(t, body, "0 * * * *")
			assert.Contains(t, body, "30 2 * * *")
			assert.Contains(t, body, "0 0 * * *")
			assert.Contains(t, body, "0 12 * * *")
			assert.Contains(t, body, ">main<")
			assert.Contains(t, body, ">develop<")
			assert.NotContains(t, body, "refs/heads/")
			assert.Contains(t, body, fmt.Sprintf("/%s/%s", repo1.OwnerName, repo1.Name))
			assert.Contains(t, body, fmt.Sprintf("/%s/%s", repo2.OwnerName, repo2.Name))
		})

		t.Run("ShowsUnparseableSpec", func(t *testing.T) {
			req := NewRequest(t, "GET", "/-/admin/actions/schedules")
			resp := adminSession.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			assert.Contains(t, body, "totally-not-a-cron-expression")
			assert.Contains(t, body, "Invalid cron expression")
		})

		t.Run("NonAdminDenied", func(t *testing.T) {
			sessionUser := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/-/admin/actions/schedules")
			sessionUser.MakeRequest(t, req, http.StatusForbidden)
		})
	})

	t.Run("Repo", func(t *testing.T) {
		repoWebURL := fmt.Sprintf("/%s/%s/settings/actions/schedules", repo1.OwnerName, repo1.Name)
		sessionOwner := loginUser(t, user2.Name)

		t.Run("PageRenders", func(t *testing.T) {
			req := NewRequest(t, "GET", repoWebURL)
			resp := sessionOwner.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			assert.True(t, test.IsNormalPageCompleted(body))
		})

		t.Run("ShowsRepoSchedulesOnly", func(t *testing.T) {
			req := NewRequest(t, "GET", repoWebURL)
			resp := sessionOwner.MakeRequest(t, req, http.StatusOK)
			body := resp.Body.String()
			assert.Contains(t, body, "ci.yml")
			assert.Contains(t, body, "deploy.yml")
			assert.Contains(t, body, "0 * * * *")
			assert.Contains(t, body, "30 2 * * *")
			assert.NotContains(t, body, "nightly.yml")
			assert.NotContains(t, body, "0 0 * * *")
			assert.NotContains(t, body, "0 12 * * *")
			assert.NotContains(t, body, ">develop<")
		})

		t.Run("SiteAdminCanAccess", func(t *testing.T) {
			req := NewRequest(t, "GET", repoWebURL)
			adminSession.MakeRequest(t, req, http.StatusOK)
		})

		t.Run("NonAdminDenied", func(t *testing.T) {
			// user4 has no access to repo1, so gets 404 (not 403) to avoid leaking existence
			sessionUser4 := loginUser(t, "user4")
			req := NewRequest(t, "GET", repoWebURL)
			sessionUser4.MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
