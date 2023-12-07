// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIAdminCreateAndDeleteSSHKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// user1 is an admin user
	session := loginUser(t, "user1")
	keyOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys?token=%s", keyOwner.Name, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		"title": "test-key",
	})
	resp := MakeRequest(t, req, http.StatusCreated)

	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)
	unittest.AssertExistsAndLoadBean(t, &asymkey_model.PublicKey{
		ID:          newPublicKey.ID,
		Name:        newPublicKey.Title,
		Fingerprint: newPublicKey.Fingerprint,
		OwnerID:     keyOwner.ID,
	})

	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d?token=%s",
		keyOwner.Name, newPublicKey.ID, token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &asymkey_model.PublicKey{ID: newPublicKey.ID})
}

func TestAPIAdminDeleteMissingSSHKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user1 is an admin user
	token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteAdmin)
	req := NewRequestf(t, "DELETE", "/api/v1/admin/users/user1/keys/%d?token=%s", unittest.NonexistentID, token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIAdminDeleteUnauthorizedKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys?token=%s", adminUsername, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		"title": "test-key",
	})
	resp := MakeRequest(t, req, http.StatusCreated)
	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)

	token = getUserToken(t, normalUsername)
	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d?token=%s",
		adminUsername, newPublicKey.ID, token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPISudoUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeReadUser)

	urlStr := fmt.Sprintf("/api/v1/user?sudo=%s&token=%s", normalUsername, token)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)
	var user api.User
	DecodeJSON(t, resp, &user)

	assert.Equal(t, normalUsername, user.UserName)
}

func TestAPISudoUserForbidden(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"

	token := getUserToken(t, normalUsername, auth_model.AccessTokenScopeReadAdmin)
	urlStr := fmt.Sprintf("/api/v1/user?sudo=%s&token=%s", adminUsername, token)
	req := NewRequest(t, "GET", urlStr)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIListUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeReadAdmin)

	urlStr := fmt.Sprintf("/api/v1/admin/users?token=%s", token)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)
	var users []api.User
	DecodeJSON(t, resp, &users)

	found := false
	for _, user := range users {
		if user.UserName == adminUsername {
			found = true
		}
	}
	assert.True(t, found)
	numberOfUsers := unittest.GetCount(t, &user_model.User{}, "type = 0")
	assert.Len(t, users, numberOfUsers)
}

func TestAPIListUsersNotLoggedIn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/api/v1/admin/users")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestAPIListUsersNonAdmin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	nonAdminUsername := "user2"
	token := getUserToken(t, nonAdminUsername)
	req := NewRequestf(t, "GET", "/api/v1/admin/users?token=%s", token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPICreateUserInvalidEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users?token=%s", token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"email":                "invalid_email@domain.com\r\n",
		"full_name":            "invalid user",
		"login_name":           "invalidUser",
		"must_change_password": "true",
		"password":             "password",
		"send_notify":          "true",
		"source_id":            "0",
		"username":             "invalidUser",
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPICreateAndDeleteUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	req := NewRequestWithValues(
		t,
		"POST",
		fmt.Sprintf("/api/v1/admin/users?token=%s", token),
		map[string]string{
			"email":                "deleteme@domain.com",
			"full_name":            "delete me",
			"login_name":           "deleteme",
			"must_change_password": "true",
			"password":             "password",
			"send_notify":          "true",
			"source_id":            "0",
			"username":             "deleteme",
		},
	)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/users/deleteme?token=%s", token))
	MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIEditUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s?token=%s", "user2", token)

	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		// required
		"login_name": "user2",
		"source_id":  "0",
		// to change
		"full_name": "Full Name User 2",
	})
	MakeRequest(t, req, http.StatusOK)

	empty := ""
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		LoginName: "user2",
		SourceID:  0,
		Email:     &empty,
	})
	resp := MakeRequest(t, req, http.StatusUnprocessableEntity)

	errMap := make(map[string]any)
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.EqualValues(t, "email is not allowed to be empty string", errMap["message"].(string))

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "user2"})
	assert.False(t, user2.IsRestricted)
	bTrue := true
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		// required
		LoginName: "user2",
		SourceID:  0,
		// to change
		Restricted: &bTrue,
	})
	MakeRequest(t, req, http.StatusOK)
	user2 = unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "user2"})
	assert.True(t, user2.IsRestricted)
}

func TestAPICreateRepoForUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	req := NewRequestWithJSON(
		t,
		"POST",
		fmt.Sprintf("/api/v1/admin/users/%s/repos?token=%s", adminUsername, token),
		&api.CreateRepoOption{
			Name: "admincreatedrepo",
		},
	)
	MakeRequest(t, req, http.StatusCreated)
}

func TestAPIRenameUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/rename?token=%s", "user2", token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "User2",
	})
	MakeRequest(t, req, http.StatusOK)

	urlStr = fmt.Sprintf("/api/v1/admin/users/%s/rename?token=%s", "User2", token)
	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "User2-2-2",
	})
	MakeRequest(t, req, http.StatusOK)

	urlStr = fmt.Sprintf("/api/v1/admin/users/%s/rename?token=%s", "User2", token)
	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "user1",
	})
	// the old user name still be used by with a redirect
	MakeRequest(t, req, http.StatusTemporaryRedirect)

	urlStr = fmt.Sprintf("/api/v1/admin/users/%s/rename?token=%s", "User2-2-2", token)
	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "user1",
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	urlStr = fmt.Sprintf("/api/v1/admin/users/%s/rename?token=%s", "User2-2-2", token)
	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "user2",
	})
	MakeRequest(t, req, http.StatusOK)
}

func TestAPICron(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user1 is an admin user
	session := loginUser(t, "user1")

	t.Run("List", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin)
		urlStr := fmt.Sprintf("/api/v1/admin/cron?token=%s", token)
		req := NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "28", resp.Header().Get("X-Total-Count"))

		var crons []api.Cron
		DecodeJSON(t, resp, &crons)
		assert.Len(t, crons, 28)
	})

	t.Run("Execute", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		now := time.Now()
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin)
		// Archive cleanup is harmless, because in the test environment there are none
		// and is thus an NOOP operation and therefore doesn't interfere with any other
		// tests.
		urlStr := fmt.Sprintf("/api/v1/admin/cron/archive_cleanup?token=%s", token)
		req := NewRequest(t, "POST", urlStr)
		MakeRequest(t, req, http.StatusNoContent)

		// Check for the latest run time for this cron, to ensure it has been run.
		urlStr = fmt.Sprintf("/api/v1/admin/cron?token=%s", token)
		req = NewRequest(t, "GET", urlStr)
		resp := MakeRequest(t, req, http.StatusOK)

		var crons []api.Cron
		DecodeJSON(t, resp, &crons)

		for _, cron := range crons {
			if cron.Name == "archive_cleanup" {
				assert.True(t, now.Before(cron.Prev))
			}
		}
	})
}
