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
	ownerSession := loginUser(t, user2.Name)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	archivedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 51})
	orgRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

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
			Specs:         []string{"30 2 * * *", "totally-not-a-cron-expression", "0 0 30 2 *"},
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
		{
			Title:         "add org workflow",
			Specs:         []string{"0 3 * * *"},
			RepoID:        orgRepo.ID,
			OwnerID:       orgRepo.OwnerID,
			WorkflowID:    "org.yml",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/master",
			CommitSHA:     "mno345",
		},
		{
			Title:         "add archived workflow",
			Specs:         []string{"0 6 * * *"},
			RepoID:        archivedRepo.ID,
			OwnerID:       archivedRepo.OwnerID,
			WorkflowID:    "archived.yml",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/master",
			CommitSHA:     "jkl012",
		},
	}))

	t.Run("Admin", func(t *testing.T) {
		req := NewRequest(t, "GET", "/-/admin/actions/schedules")
		resp := adminSession.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()

		t.Run("PageRenders", func(t *testing.T) {
			assert.True(t, test.IsNormalPageCompleted(body))
		})

		t.Run("ShowsAllSchedules", func(t *testing.T) {
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

		t.Run("WarnsAboutSchedulesThatNeverRun", func(t *testing.T) {
			assert.Contains(t, body, "totally-not-a-cron-expression")
			assert.Contains(t, body, "Invalid cron expression")
			// parses, but Feb 30 never comes, so the spec has no usable next run
			assert.Contains(t, body, "no valid next run")
			assert.Contains(t, body, "Repository is archived")
		})

		t.Run("NonAdminDenied", func(t *testing.T) {
			req := NewRequest(t, "GET", "/-/admin/actions/schedules")
			ownerSession.MakeRequest(t, req, http.StatusForbidden)
		})
	})

	t.Run("Repo", func(t *testing.T) {
		repoWebURL := fmt.Sprintf("/%s/%s/settings/actions/schedules", repo1.OwnerName, repo1.Name)

		req := NewRequest(t, "GET", repoWebURL)
		resp := ownerSession.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()

		t.Run("PageRenders", func(t *testing.T) {
			assert.True(t, test.IsNormalPageCompleted(body))
		})

		t.Run("ShowsRepoSchedulesOnly", func(t *testing.T) {
			assert.Contains(t, body, "ci.yml")
			assert.Contains(t, body, "deploy.yml")
			assert.Contains(t, body, "0 * * * *")
			assert.Contains(t, body, "30 2 * * *")
			assert.NotContains(t, body, "nightly.yml")
			assert.NotContains(t, body, "0 0 * * *")
			assert.NotContains(t, body, "0 12 * * *")
			assert.NotContains(t, body, ">develop<")
		})

		t.Run("SiteAdminSeesRepoScopeOnly", func(t *testing.T) {
			req := NewRequest(t, "GET", repoWebURL)
			resp := adminSession.MakeRequest(t, req, http.StatusOK)
			// repo scope must win over admin scope, otherwise every repo's schedules leak onto this page
			assert.NotContains(t, resp.Body.String(), "nightly.yml")
		})

		t.Run("NonAdminDenied", func(t *testing.T) {
			// RequireRepoAdmin rejects a user who can read repo1 but cannot administer it
			sessionUser4 := loginUser(t, "user4")
			req := NewRequest(t, "GET", repoWebURL)
			sessionUser4.MakeRequest(t, req, http.StatusNotFound)
		})
	})

	t.Run("User", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user/settings/actions/schedules")
		resp := ownerSession.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.True(t, test.IsNormalPageCompleted(body))
		// owner scope spans every repository of the owner
		assert.Contains(t, body, "ci.yml")
		assert.Contains(t, body, "nightly.yml")
		assert.NotContains(t, body, "org.yml")
	})

	t.Run("Org", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/org/%s/settings/actions/schedules", orgRepo.OwnerName))
		resp := ownerSession.MakeRequest(t, req, http.StatusOK)
		body := resp.Body.String()
		assert.True(t, test.IsNormalPageCompleted(body))
		assert.Contains(t, body, "org.yml")
		assert.NotContains(t, body, "ci.yml")
	})
}
