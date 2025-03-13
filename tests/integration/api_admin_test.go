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
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
)

func TestAPIAdminCreateAndDeleteSSHKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// user1 is an admin user
	session := loginUser(t, "user1")
	keyOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys", keyOwner.Name)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		"title": "test-key",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)
	unittest.AssertExistsAndLoadBean(t, &asymkey_model.PublicKey{
		ID:          newPublicKey.ID,
		Name:        newPublicKey.Title,
		Fingerprint: newPublicKey.Fingerprint,
		OwnerID:     keyOwner.ID,
	})

	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d", keyOwner.Name, newPublicKey.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &asymkey_model.PublicKey{ID: newPublicKey.ID})
}

func TestAPIAdminDeleteMissingSSHKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user1 is an admin user
	token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteAdmin)
	req := NewRequestf(t, "DELETE", "/api/v1/admin/users/user1/keys/%d", unittest.NonexistentID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIAdminDeleteUnauthorizedKey(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys", adminUsername)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		"title": "test-key",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)

	token = getUserToken(t, normalUsername, auth_model.AccessTokenScopeAll)
	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d", adminUsername, newPublicKey.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPISudoUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeReadUser)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user?sudo=%s", normalUsername)).
		AddTokenAuth(token)
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
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user?sudo=%s", adminUsername)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIListUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeReadAdmin)

	req := NewRequest(t, "GET", "/api/v1/admin/users").
		AddTokenAuth(token)
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
	token := getUserToken(t, nonAdminUsername, auth_model.AccessTokenScopeAll)
	req := NewRequest(t, "GET", "/api/v1/admin/users").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPICreateUserInvalidEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	req := NewRequestWithValues(t, "POST", "/api/v1/admin/users", map[string]string{
		"email":                "invalid_email@domain.com\r\n",
		"full_name":            "invalid user",
		"login_name":           "invalidUser",
		"must_change_password": "true",
		"password":             "password",
		"send_notify":          "true",
		"source_id":            "0",
		"username":             "invalidUser",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPICreateAndDeleteUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	req := NewRequestWithValues(
		t,
		"POST",
		"/api/v1/admin/users",
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
	).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequest(t, "DELETE", "/api/v1/admin/users/deleteme").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIEditUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s", "user2")

	fullNameToChange := "Full Name User 2"
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		// required
		"login_name": "user2",
		"source_id":  "0",
		// to change
		"full_name": fullNameToChange,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "user2"})
	assert.Equal(t, fullNameToChange, user2.FullName)

	empty := ""
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		LoginName: "user2",
		SourceID:  0,
		Email:     &empty,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusBadRequest)

	errMap := make(map[string]any)
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.EqualValues(t, "e-mail invalid [email: ]", errMap["message"].(string))

	user2 = unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "user2"})
	assert.False(t, user2.IsRestricted)
	bTrue := true
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		// required
		LoginName: "user2",
		SourceID:  0,
		// to change
		Restricted: &bTrue,
	}).AddTokenAuth(token)
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
		fmt.Sprintf("/api/v1/admin/users/%s/repos", adminUsername),
		&api.CreateRepoOption{
			Name: "admincreatedrepo",
		},
	).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func TestAPIRenameUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/rename", "user2")
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "User2",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	urlStr = fmt.Sprintf("/api/v1/admin/users/%s/rename", "User2")
	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "User2-2-2",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "user1",
	}).AddTokenAuth(token)
	// the old user name still be used by with a redirect
	MakeRequest(t, req, http.StatusTemporaryRedirect)

	urlStr = fmt.Sprintf("/api/v1/admin/users/%s/rename", "User2-2-2")
	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "user1",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
		// required
		"new_name": "user2",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}

func TestAPICron(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user1 is an admin user
	session := loginUser(t, "user1")

	t.Run("List", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin)

		req := NewRequest(t, "GET", "/api/v1/admin/cron").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "29", resp.Header().Get("X-Total-Count"))

		var crons []api.Cron
		DecodeJSON(t, resp, &crons)
		assert.Len(t, crons, 29)
	})

	t.Run("Execute", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		now := time.Now()
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin)
		// Archive cleanup is harmless, because in the test environment there are none
		// and is thus an NOOP operation and therefore doesn't interfere with any other
		// tests.
		req := NewRequest(t, "POST", "/api/v1/admin/cron/archive_cleanup").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Check for the latest run time for this cron, to ensure it has been run.
		req = NewRequest(t, "GET", "/api/v1/admin/cron").
			AddTokenAuth(token)
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

func TestAPICreateUser_NotAllowedEmailDomain(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.Service.EmailDomainAllowList = []glob.Glob{glob.MustCompile("example.org")}
	defer func() {
		setting.Service.EmailDomainAllowList = []glob.Glob{}
	}()

	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	req := NewRequestWithValues(t, "POST", "/api/v1/admin/users", map[string]string{
		"email":                "allowedUser1@example1.org",
		"login_name":           "allowedUser1",
		"username":             "allowedUser1",
		"password":             "allowedUser1_pass",
		"must_change_password": "true",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	assert.Equal(t, "the domain of user email allowedUser1@example1.org conflicts with EMAIL_DOMAIN_ALLOWLIST or EMAIL_DOMAIN_BLOCKLIST", resp.Header().Get("X-Gitea-Warning"))

	req = NewRequest(t, "DELETE", "/api/v1/admin/users/allowedUser1").AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIEditUser_NotAllowedEmailDomain(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.Service.EmailDomainAllowList = []glob.Glob{glob.MustCompile("example.org")}
	defer func() {
		setting.Service.EmailDomainAllowList = []glob.Glob{}
	}()

	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s", "user2")

	newEmail := "user2@example1.com"
	req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		LoginName: "user2",
		SourceID:  0,
		Email:     &newEmail,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "the domain of user email user2@example1.com conflicts with EMAIL_DOMAIN_ALLOWLIST or EMAIL_DOMAIN_BLOCKLIST", resp.Header().Get("X-Gitea-Warning"))

	originalEmail := "user2@example.com"
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		LoginName: "user2",
		SourceID:  0,
		Email:     &originalEmail,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
}
