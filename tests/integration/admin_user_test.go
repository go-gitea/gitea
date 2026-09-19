// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminViewUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/-/admin/users")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/-/admin/users")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAdminViewUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/-/admin/users/1")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/-/admin/users/1")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAdminEditUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testSuccessfulEdit(t, user_model.User{ID: 2, Name: "newusername", LoginName: "otherlogin", Email: "new@e-mail.gitea"})
}

func testSuccessfulEdit(t *testing.T, formData user_model.User) {
	makeRequest(t, formData, http.StatusSeeOther)
}

func makeRequest(t *testing.T, formData user_model.User, headerCode int) {
	session := loginUser(t, "user1")
	req := NewRequestWithValues(t, "POST", "/-/admin/users/"+strconv.Itoa(int(formData.ID))+"/edit", map[string]string{
		"user_name":  formData.Name,
		"login_name": formData.LoginName,
		"login_type": "0-0",
		"email":      formData.Email,
	})

	session.MakeRequest(t, req, headerCode)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: formData.ID})
	assert.Equal(t, formData.Name, user.Name)
	assert.Equal(t, formData.LoginName, user.LoginName)
	assert.Equal(t, formData.Email, user.Email)
}

func TestAdminDeleteUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	usersToDelete := []struct {
		userID int64
		purge  bool
	}{
		{
			userID: 2,
			purge:  true,
		},
		{
			userID: 8,
		},
	}

	for _, entry := range usersToDelete {
		t.Run(fmt.Sprintf("DeleteUser%d", entry.userID), func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: entry.userID})
			assert.NotNil(t, user)

			var query string
			if entry.purge {
				query = "?purge=true"
			}

			req := NewRequest(t, "POST", fmt.Sprintf("/-/admin/users/%d/delete%s", entry.userID, query))
			session.MakeRequest(t, req, http.StatusSeeOther)

			assertUserDeleted(t, entry.userID)
			unittest.CheckConsistencyFor(t, &user_model.User{})
		})
	}
}

func TestAdminImpersonatedUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 never signed in yet, only the user themselves should be asked to set a password
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2.MustChangePassword = true
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user2, "must_change_password"))

	session := loginUser(t, "user1")
	homeDoc := func(t *testing.T) *HTMLDoc {
		t.Helper()
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		return NewHTMLParser(t, resp.Body)
	}
	currentUsername := func(doc *HTMLDoc) string {
		return doc.Find("[data-signed-in-username]").AttrOr("data-signed-in-username", "")
	}

	// user1 is admin, can visit admin pages
	assert.Equal(t, "user1", currentUsername(homeDoc(t)))
	assert.Equal(t, 0, homeDoc(t).Find(".site-banner-container").Length())
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2"), http.StatusOK)

	// impersonate to user2, user2 can't visit admin pages
	session.MakeRequest(t, NewRequest(t, "POST", "/-/admin/users/2/impersonate"), http.StatusOK)
	doc := homeDoc(t)
	assert.Equal(t, "user2", currentUsername(doc))
	assert.Contains(t, doc.Find(".site-banner-container").Text(), "user2")
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2"), http.StatusForbidden)
	// the impersonating admin must not set the password of the impersonated user
	session.MakeRequest(t, NewRequest(t, "GET", "/user/settings/change_password"), http.StatusSeeOther)

	// exit impersonation, current user is user1(admin) again
	session.MakeRequest(t, NewRequest(t, "GET", "/user/logout"), http.StatusSeeOther)
	assert.Equal(t, "user1", currentUsername(homeDoc(t)))
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2"), http.StatusOK)

	// completely logout
	session.MakeRequest(t, NewRequest(t, "GET", "/user/logout"), http.StatusSeeOther)
	assert.Equal(t, "", currentUsername(homeDoc(t)))
}

func TestAdminBotUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	t.Run("CreateWithoutPassword", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", "/-/admin/users/new", map[string]string{
			"user_type":  "Bot",
			"login_type": "0-0",
			"user_name":  "bot-user",
			"email":      "bot-user@example.com",
			"visibility": "0",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		assert.True(t, bot.IsTypeBot())
		assert.Empty(t, bot.Passwd)
		assert.False(t, bot.MustChangePassword)

		doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/-/admin/users/%d/edit", bot.ID)), http.StatusOK).Body)
		assert.Equal(t, "Bot", doc.Find("#user_type").AttrOr("value", ""))
		assert.Empty(t, doc.Find("#login_type").Nodes)
		assert.Empty(t, doc.Find("#password").Nodes)
		doc = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/-/admin/users/%d", bot.ID)), http.StatusOK).Body)
		assert.NotEmpty(t, doc.Find(`form[action$="/access_tokens"]`).Nodes)
	})

	t.Run("EditWithoutAuthSource", func(t *testing.T) {
		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/-/admin/users/%d/edit", bot.ID), map[string]string{
			"user_name":  "bot-user",
			"login_type": "0-0",
			"email":      "bot-user@example.com",
			"full_name":  "Bot User",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		assert.Equal(t, "Bot User", unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: bot.ID}).FullName)
	})

	t.Run("TokenScope", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()
		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		tokenURL := fmt.Sprintf("/-/admin/users/%d/access_tokens", bot.ID)

		resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", tokenURL, map[string]string{
			"name": "no-scope",
		}), http.StatusBadRequest)
		assert.Contains(t, resp.Body.String(), "at least one permission")
		assert.Equal(t, 0, unittest.GetCount(t, &auth_model.AccessToken{UID: bot.ID}))

		resp = session.MakeRequest(t, NewRequestWithValues(t, "POST", tokenURL, map[string]string{
			"name":             "ci",
			"scope-repository": "write:repository",
		}), http.StatusOK)
		panel := NewHTMLParser(t, resp.Body)
		assert.NotEmpty(t, panel.Find("#new-access-token-value").Text())
		assert.Equal(t, 1, panel.Find(`[data-clipboard-target="#new-access-token-value"]`).Length())
		assert.Equal(t, tokenURL, panel.Find("form.form-fetch-action").AttrOr("action", ""))
		assert.Equal(t, 1, unittest.GetCount(t, &auth_model.AccessToken{UID: bot.ID}))

		resp = session.MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/users/2/access_tokens", map[string]string{
			"name":             "not-a-bot",
			"scope-repository": "write:repository",
		}), http.StatusBadRequest)
		assert.Contains(t, resp.Body.String(), "only be generated for bot accounts")
		unittest.AssertNotExistsBean(t, &auth_model.AccessToken{UID: 2, Name: "not-a-bot"})

		token := unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{UID: bot.ID, Name: "ci"})
		session.MakeRequest(t, NewRequestWithValues(t, "POST", tokenURL+"/delete", map[string]string{
			"id": strconv.FormatInt(token.ID, 10),
		}), http.StatusOK)
		assert.Equal(t, 0, unittest.GetCount(t, &auth_model.AccessToken{UID: bot.ID}))

		for _, action := range []audit_model.Action{audit_model.UserAccessTokenAdd, audit_model.UserAccessTokenRemove} {
			events, _, err := audit_model.FindEvents(t.Context(), &audit_model.EventSearchOptions{Action: action, ScopeType: audit_model.ScopeUser, ScopeID: bot.ID})
			require.NoError(t, err)
			require.Len(t, events, 1, "audit events for %s", action)
			assert.Equal(t, int64(1), events[0].ActorID)
			assert.Equal(t, "ci", audit_model.DecodeMetadata(events[0].Metadata)["token"])
		}
	})

	t.Run("APIRejectsAuthSource", func(t *testing.T) {
		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/admin/users/"+bot.Name, map[string]any{"source_id": 1}).AddBasicAuth("user1")
		MakeRequest(t, req, http.StatusBadRequest)

		bot = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: bot.ID})
		assert.True(t, bot.IsLocal())
		assert.Empty(t, bot.LoginName)
	})

	t.Run("ConvertType", func(t *testing.T) {
		editUserType := func(userID int64, userType string) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
			session.MakeRequest(t, NewRequestWithValues(t, "POST", fmt.Sprintf("/-/admin/users/%d/edit", userID), map[string]string{
				"user_name":  user.Name,
				"login_type": "0-0",
				"login_name": user.LoginName,
				"password":   "Bot-Password-1234",
				"email":      user.Email,
				"user_type":  userType,
				"visibility": "0",
			}), http.StatusSeeOther)
		}

		MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/admin/users/org3", map[string]string{"type": "Organization"}).AddBasicAuth("user1"), http.StatusOK)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/admin/users/user4", map[string]string{"type": "Bot", "password": "Bot-Password-1234"}).AddBasicAuth("user1"), http.StatusBadRequest)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/admin/users/user4", map[string]string{"type": "Bot"}).AddBasicAuth("user1"), http.StatusOK)
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		assert.True(t, user4.IsTypeBot())
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/users/user4"), http.StatusOK)
		assert.Equal(t, api.UserTypeStringBot, DecodeJSON(t, resp, &api.User{}).Type)
		session.MakeRequest(t, NewRequest(t, "POST", "/-/admin/users/4/impersonate"), http.StatusBadRequest)

		editUserType(4, "User")
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).IsIndividual())

		editUserType(4, "Bot")
		converted := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		assert.True(t, converted.IsTypeBot())
		assert.Equal(t, user4.Passwd, converted.Passwd)
	})
}
