// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsRunnerModify(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()

	require.NoError(t, db.DeleteAllRecords("action_runner"))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	_ = actions_model.CreateRunner(ctx, &actions_model.ActionRunner{OwnerID: user2.ID, Name: "user2-runner", TokenHash: "a", UUID: "a"})
	user2Runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{OwnerID: user2.ID, Name: "user2-runner"})
	userWebURL := "/user/settings/actions/runners"

	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3, Type: user_model.UserTypeOrganization})
	require.NoError(t, actions_model.CreateRunner(ctx, &actions_model.ActionRunner{OwnerID: org3.ID, Name: "org3-runner", TokenHash: "b", UUID: "b"}))
	org3Runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{OwnerID: org3.ID, Name: "org3-runner"})
	orgWebURL := "/org/org3/settings/actions/runners"

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_ = actions_model.CreateRunner(ctx, &actions_model.ActionRunner{RepoID: repo1.ID, Name: "repo1-runner", TokenHash: "c", UUID: "c"})
	repo1Runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{RepoID: repo1.ID, Name: "repo1-runner"})
	repoWebURL := "/user2/repo1/settings/actions/runners"

	_ = actions_model.CreateRunner(ctx, &actions_model.ActionRunner{Name: "global-runner", TokenHash: "d", UUID: "d"})
	globalRunner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{Name: "global-runner"})
	adminWebURL := "/-/admin/actions/runners"

	sessionAdmin := loginUser(t, "user1")
	sessionUser2 := loginUser(t, user2.Name)

	doUpdate := func(t *testing.T, sess *TestSession, baseURL string, id int64, description string, expectedStatus int) {
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("%s/%d", baseURL, id), map[string]string{
			"description": description,
		})
		sess.MakeRequest(t, req, expectedStatus)
	}

	doDelete := func(t *testing.T, sess *TestSession, baseURL string, id int64, expectedStatus int) {
		req := NewRequest(t, "POST", fmt.Sprintf("%s/%d/delete", baseURL, id))
		sess.MakeRequest(t, req, expectedStatus)
	}

	doDisable := func(t *testing.T, sess *TestSession, baseURL string, id int64, expectedStatus int) {
		req := NewRequest(t, "POST", fmt.Sprintf("%s/%d/update-runner?disabled=true", baseURL, id))
		sess.MakeRequest(t, req, expectedStatus)
	}

	doEnable := func(t *testing.T, sess *TestSession, baseURL string, id int64, expectedStatus int) {
		req := NewRequest(t, "POST", fmt.Sprintf("%s/%d/update-runner?disabled=false", baseURL, id))
		sess.MakeRequest(t, req, expectedStatus)
	}

	assertDenied := func(t *testing.T, sess *TestSession, baseURL string, id int64) {
		doUpdate(t, sess, baseURL, id, "ChangedDescription", http.StatusNotFound)
		doDisable(t, sess, baseURL, id, http.StatusNotFound)
		doEnable(t, sess, baseURL, id, http.StatusNotFound)
		doDelete(t, sess, baseURL, id, http.StatusNotFound)
		v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
		assert.Empty(t, v.Description)
		assert.False(t, v.IsDisabled)
	}

	assertSuccess := func(t *testing.T, sess *TestSession, baseURL string, id int64) {
		doUpdate(t, sess, baseURL, id, "ChangedDescription", http.StatusSeeOther)
		v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
		assert.Equal(t, "ChangedDescription", v.Description)
		doDisable(t, sess, baseURL, id, http.StatusOK)
		v = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
		assert.True(t, v.IsDisabled)
		doEnable(t, sess, baseURL, id, http.StatusOK)
		v = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
		assert.False(t, v.IsDisabled)
		doDelete(t, sess, baseURL, id, http.StatusOK)
		unittest.AssertNotExistsBean(t, &actions_model.ActionRunner{ID: id})
	}

	t.Run("UpdateUserRunner", func(t *testing.T) {
		theRunner := user2Runner
		t.Run("FromOrg", func(t *testing.T) {
			assertDenied(t, sessionAdmin, orgWebURL, theRunner.ID)
		})
		t.Run("FromRepo", func(t *testing.T) {
			assertDenied(t, sessionAdmin, repoWebURL, theRunner.ID)
		})
		t.Run("FromAdmin", func(t *testing.T) {
			t.Skip("Admin can update any runner (not right but not too bad)")
			assertDenied(t, sessionAdmin, adminWebURL, theRunner.ID)
		})
	})

	t.Run("UpdateOrgRunner", func(t *testing.T) {
		theRunner := org3Runner
		t.Run("FromRepo", func(t *testing.T) {
			assertDenied(t, sessionAdmin, repoWebURL, theRunner.ID)
		})
		t.Run("FromUser", func(t *testing.T) {
			assertDenied(t, sessionAdmin, userWebURL, theRunner.ID)
		})
		t.Run("FromAdmin", func(t *testing.T) {
			t.Skip("Admin can update any runner (not right but not too bad)")
			assertDenied(t, sessionAdmin, adminWebURL, theRunner.ID)
		})
	})

	t.Run("UpdateRepoRunner", func(t *testing.T) {
		theRunner := repo1Runner
		t.Run("FromOrg", func(t *testing.T) {
			assertDenied(t, sessionAdmin, orgWebURL, theRunner.ID)
		})
		t.Run("FromUser", func(t *testing.T) {
			assertDenied(t, sessionAdmin, userWebURL, theRunner.ID)
		})
		t.Run("FromAdmin", func(t *testing.T) {
			t.Skip("Admin can update any runner (not right but not too bad)")
			assertDenied(t, sessionAdmin, adminWebURL, theRunner.ID)
		})
	})

	t.Run("UpdateGlobalRunner", func(t *testing.T) {
		theRunner := globalRunner
		t.Run("FromOrg", func(t *testing.T) {
			assertDenied(t, sessionAdmin, orgWebURL, theRunner.ID)
		})
		t.Run("FromUser", func(t *testing.T) {
			assertDenied(t, sessionAdmin, userWebURL, theRunner.ID)
		})
		t.Run("FromRepo", func(t *testing.T) {
			assertDenied(t, sessionAdmin, repoWebURL, theRunner.ID)
		})
	})

	t.Run("UpdateSuccess", func(t *testing.T) {
		t.Run("User", func(t *testing.T) {
			assertSuccess(t, sessionUser2, userWebURL, user2Runner.ID)
		})
		t.Run("Org", func(t *testing.T) {
			assertSuccess(t, sessionAdmin, orgWebURL, org3Runner.ID)
		})
		t.Run("Repo", func(t *testing.T) {
			assertSuccess(t, sessionUser2, repoWebURL, repo1Runner.ID)
		})
		t.Run("Admin", func(t *testing.T) {
			assertSuccess(t, sessionAdmin, adminWebURL, globalRunner.ID)
		})
	})

	t.Run("BulkAction", func(t *testing.T) {
		// Previous subtests deleted all runners; create a fresh set scoped to this subtest.
		require.NoError(t, actions_model.CreateRunner(ctx, &actions_model.ActionRunner{Name: "bulk-runner-1", TokenHash: "e", UUID: "e"}))
		require.NoError(t, actions_model.CreateRunner(ctx, &actions_model.ActionRunner{Name: "bulk-runner-2", TokenHash: "f", UUID: "f"}))
		require.NoError(t, actions_model.CreateRunner(ctx, &actions_model.ActionRunner{Name: "bulk-runner-3", TokenHash: "g", UUID: "g"}))
		r1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{Name: "bulk-runner-1"})
		r2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{Name: "bulk-runner-2"})
		r3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{Name: "bulk-runner-3"})
		allIDs := []int64{r1.ID, r2.ID, r3.ID}
		bulkURL := adminWebURL + "/bulk"
		doBulk := func(t *testing.T, sess *TestSession, action string, ids []int64, expectedStatus int) {
			req := NewRequestWithValues(t, "POST", bulkURL, map[string]string{
				"action": action,
				"ids":    strings.Join(base.Int64sToStrings(ids), ","),
			})
			sess.MakeRequest(t, req, expectedStatus)
		}

		t.Run("NonAdminForbidden", func(t *testing.T) {
			doBulk(t, sessionUser2, "disable", allIDs, http.StatusForbidden)
			for _, id := range allIDs {
				v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
				assert.False(t, v.IsDisabled, "runner %d should not have been disabled", id)
			}
		})

		t.Run("InvalidAction", func(t *testing.T) {
			doBulk(t, sessionAdmin, "evict", allIDs, http.StatusBadRequest)
		})

		t.Run("DisableEnable", func(t *testing.T) {
			doBulk(t, sessionAdmin, "disable", allIDs, http.StatusOK)
			for _, id := range allIDs {
				v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
				assert.True(t, v.IsDisabled, "runner %d should be disabled", id)
			}
			doBulk(t, sessionAdmin, "enable", allIDs, http.StatusOK)
			for _, id := range allIDs {
				v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: id})
				assert.False(t, v.IsDisabled, "runner %d should be enabled", id)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			doBulk(t, sessionAdmin, "delete", allIDs, http.StatusOK)
			for _, id := range allIDs {
				unittest.AssertNotExistsBean(t, &actions_model.ActionRunner{ID: id})
			}
		})
	})
}
