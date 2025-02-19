// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsVariables(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()

	require.NoError(t, db.DeleteAllRecords("action_variable"))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	_, _ = actions_model.InsertVariable(ctx, user2.ID, 0, "VAR", "user2-var")
	user2Var := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{OwnerID: user2.ID, Name: "VAR"})
	userWebURL := "/user/settings/actions/variables"

	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3, Type: user_model.UserTypeOrganization})
	_, _ = actions_model.InsertVariable(ctx, org3.ID, 0, "VAR", "org3-var")
	org3Var := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{OwnerID: org3.ID, Name: "VAR"})
	orgWebURL := "/org/org3/settings/actions/variables"

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_, _ = actions_model.InsertVariable(ctx, 0, repo1.ID, "VAR", "repo1-var")
	repo1Var := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{RepoID: repo1.ID, Name: "VAR"})
	repoWebURL := "/user2/repo1/settings/actions/variables"

	_, _ = actions_model.InsertVariable(ctx, 0, 0, "VAR", "global-var")
	globalVar := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{Name: "VAR", Data: "global-var"})
	adminWebURL := "/-/admin/actions/variables"

	sessionAdmin := loginUser(t, "user1")
	sessionUser2 := loginUser(t, user2.Name)

	doUpdate := func(t *testing.T, sess *TestSession, baseURL string, id int64, data string, expectedStatus int) {
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("%s/%d/edit", baseURL, id), map[string]string{
			"_csrf": GetUserCSRFToken(t, sess),
			"name":  "VAR",
			"data":  data,
		})
		sess.MakeRequest(t, req, expectedStatus)
	}

	doDelete := func(t *testing.T, sess *TestSession, baseURL string, id int64, expectedStatus int) {
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("%s/%d/delete", baseURL, id), map[string]string{
			"_csrf": GetUserCSRFToken(t, sess),
		})
		sess.MakeRequest(t, req, expectedStatus)
	}

	assertDenied := func(t *testing.T, sess *TestSession, baseURL string, id int64) {
		doUpdate(t, sess, baseURL, id, "ChangedData", http.StatusNotFound)
		doDelete(t, sess, baseURL, id, http.StatusNotFound)
		v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{ID: id})
		assert.Contains(t, v.Data, "-var")
	}

	assertSuccess := func(t *testing.T, sess *TestSession, baseURL string, id int64) {
		doUpdate(t, sess, baseURL, id, "ChangedData", http.StatusOK)
		v := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{ID: id})
		assert.Equal(t, "ChangedData", v.Data)
		doDelete(t, sess, baseURL, id, http.StatusOK)
		unittest.AssertNotExistsBean(t, &actions_model.ActionVariable{ID: id})
	}

	t.Run("UpdateUserVar", func(t *testing.T) {
		theVar := user2Var
		t.Run("FromOrg", func(t *testing.T) {
			assertDenied(t, sessionAdmin, orgWebURL, theVar.ID)
		})
		t.Run("FromRepo", func(t *testing.T) {
			assertDenied(t, sessionAdmin, repoWebURL, theVar.ID)
		})
		t.Run("FromAdmin", func(t *testing.T) {
			assertDenied(t, sessionAdmin, adminWebURL, theVar.ID)
		})
	})

	t.Run("UpdateOrgVar", func(t *testing.T) {
		theVar := org3Var
		t.Run("FromRepo", func(t *testing.T) {
			assertDenied(t, sessionAdmin, repoWebURL, theVar.ID)
		})
		t.Run("FromUser", func(t *testing.T) {
			assertDenied(t, sessionAdmin, userWebURL, theVar.ID)
		})
		t.Run("FromAdmin", func(t *testing.T) {
			assertDenied(t, sessionAdmin, adminWebURL, theVar.ID)
		})
	})

	t.Run("UpdateRepoVar", func(t *testing.T) {
		theVar := repo1Var
		t.Run("FromOrg", func(t *testing.T) {
			assertDenied(t, sessionAdmin, orgWebURL, theVar.ID)
		})
		t.Run("FromUser", func(t *testing.T) {
			assertDenied(t, sessionAdmin, userWebURL, theVar.ID)
		})
		t.Run("FromAdmin", func(t *testing.T) {
			assertDenied(t, sessionAdmin, adminWebURL, theVar.ID)
		})
	})

	t.Run("UpdateGlobalVar", func(t *testing.T) {
		theVar := globalVar
		t.Run("FromOrg", func(t *testing.T) {
			assertDenied(t, sessionAdmin, orgWebURL, theVar.ID)
		})
		t.Run("FromUser", func(t *testing.T) {
			assertDenied(t, sessionAdmin, userWebURL, theVar.ID)
		})
		t.Run("FromRepo", func(t *testing.T) {
			assertDenied(t, sessionAdmin, repoWebURL, theVar.ID)
		})
	})

	t.Run("UpdateSuccess", func(t *testing.T) {
		t.Run("User", func(t *testing.T) {
			assertSuccess(t, sessionUser2, userWebURL, user2Var.ID)
		})
		t.Run("Org", func(t *testing.T) {
			assertSuccess(t, sessionAdmin, orgWebURL, org3Var.ID)
		})
		t.Run("Repo", func(t *testing.T) {
			assertSuccess(t, sessionUser2, repoWebURL, repo1Var.ID)
		})
		t.Run("Admin", func(t *testing.T) {
			assertSuccess(t, sessionAdmin, adminWebURL, globalVar.ID)
		})
	})
}
