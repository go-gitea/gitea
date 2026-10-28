// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	asymkey_model "gitea.dev/models/asymkey"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/glob"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIAdminListPackages(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	packageName := "admin-test-package"
	packageVersion := "1.0.0"

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s/file.bin", user4.Name, packageName, packageVersion)
	req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{})).AddBasicAuth(user4.Name)
	MakeRequest(t, req, http.StatusCreated)

	adminToken := getUserToken(t, "user1", auth_model.AccessTokenScopeReadAdmin)
	req = NewRequest(t, "GET", "/api/v1/admin/packages").AddTokenAuth(adminToken)
	resp := MakeRequest(t, req, http.StatusOK)

	apiPackages := DecodeJSON(t, resp, []*api.Package{})
	actual := map[string]any{}
	for _, p := range apiPackages {
		actual[p.Name] = map[string]any{"ownerName": p.Owner.UserName, "type": p.Type, "version": p.Version}
	}
	assert.Equal(t, map[string]any{
		packageName: map[string]any{"ownerName": user4.Name, "type": "generic", "version": packageVersion},
	}, actual)
}

func TestAPIAdminAccessAndSSHKeys(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("ListUsers", func(t *testing.T) {
		adminUsername := "user1"
		token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeReadAdmin)

		req := NewRequest(t, "GET", "/api/v1/admin/users").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		users := DecodeJSON(t, resp, []api.User{})

		found := false
		for _, user := range users {
			if user.UserName == adminUsername {
				found = true
			}
		}
		assert.True(t, found)
		numberOfUsers := unittest.GetCount(t, &user_model.User{}, "type = 0")
		assert.Len(t, users, numberOfUsers)
	})

	t.Run("ListUsersNotLoggedIn", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/admin/users")
		MakeRequest(t, req, http.StatusUnauthorized)
	})

	t.Run("ListUsersNonAdmin", func(t *testing.T) {
		nonAdminUsername := "user2"
		token := getUserToken(t, nonAdminUsername, auth_model.AccessTokenScopeAll)
		req := NewRequest(t, "GET", "/api/v1/admin/users").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("SudoUser", func(t *testing.T) {
		adminUsername := "user1"
		normalUsername := "user2"
		token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeReadUser)

		req := NewRequest(t, "GET", "/api/v1/user?sudo="+normalUsername).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		user := DecodeJSON(t, resp, &api.User{})

		assert.Equal(t, normalUsername, user.UserName)
	})

	t.Run("SudoUserForbidden", func(t *testing.T) {
		adminUsername := "user1"
		normalUsername := "user2"

		token := getUserToken(t, normalUsername, auth_model.AccessTokenScopeReadAdmin)
		req := NewRequest(t, "GET", "/api/v1/user?sudo="+adminUsername).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("CreateUserInvalidEmail", func(t *testing.T) {
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
	})

	t.Run("DeleteMissingSSHKey", func(t *testing.T) {
		// user1 is an admin user
		token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteAdmin)
		req := NewRequestf(t, "DELETE", "/api/v1/admin/users/user1/keys/%d", unittest.NonexistentID).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Cron", func(t *testing.T) {
		// user1 is an admin user
		session := loginUser(t, "user1")

		t.Run("List", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin)

			req := NewRequest(t, "GET", "/api/v1/admin/cron").
				AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "30", resp.Header().Get("X-Total-Count"))

			crons := DecodeJSON(t, resp, []api.Cron{})
			assert.Len(t, crons, 30)
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

			crons := DecodeJSON(t, resp, []api.Cron{})

			for _, cron := range crons {
				if cron.Name == "archive_cleanup" {
					assert.True(t, now.Before(cron.Prev))
				}
			}
		})
	})

	t.Run("CreateAndDeleteSSHKey", func(t *testing.T) {
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

		newPublicKey := DecodeJSON(t, resp, &api.PublicKey{})
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
	})

	t.Run("DeleteUnauthorizedKey", func(t *testing.T) {
		adminUsername := "user1"
		normalUsername := "user2"
		token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

		urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys", adminUsername)
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
			"title": "test-key",
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		newPublicKey := DecodeJSON(t, resp, &api.PublicKey{})

		token = getUserToken(t, normalUsername, auth_model.AccessTokenScopeAll)
		req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d", adminUsername, newPublicKey.ID).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})
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
	urlStr := "/api/v1/admin/users/" + "user2"

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

	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{SourceID: 0, Email: new("")}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusBadRequest)

	errMap := make(map[string]any)
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.Equal(t, "email address is invalid: ", errMap["message"])

	user2 = unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "user2"})
	assert.False(t, user2.IsRestricted)
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{Restricted: new(true)}).AddTokenAuth(token)
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
	for _, testCase := range []struct {
		username string
		newName  string
		status   int
	}{
		{"user2", "User2", http.StatusNoContent},
		{"User2", "User2-2-2", http.StatusNoContent},
		// the old user name still be used by with a redirect
		{"User2", "user1", http.StatusTemporaryRedirect},
		{"User2-2-2", "user1", http.StatusUnprocessableEntity},
		{"User2-2-2", "user2", http.StatusNoContent},
	} {
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/api/v1/admin/users/%s/rename", testCase.username), map[string]string{
			// required
			"new_name": testCase.newName,
		}).AddTokenAuth(token)
		MakeRequest(t, req, testCase.status)
	}
}

func TestAPICreateUser_NotAllowedEmailDomain(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.EmailDomainAllowList, []glob.Glob{glob.MustCompile("example.org")})()

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
	defer test.MockVariableValue(&setting.Service.EmailDomainAllowList, []glob.Glob{glob.MustCompile("example.org")})()

	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	urlStr := "/api/v1/admin/users/user2"

	req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{Email: new("user2@example1.com")}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusBadRequest)
	errMap := make(map[string]string)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), &errMap))
	assert.Equal(t, "email domain is not allowed: user2@example1.com", errMap["message"])

	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{Email: new("user2@example.org")}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
}
