// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
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
			"user_type":  "bot",
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

		// a bot has no auth source or password to edit, but its access tokens are managed by the admin
		doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/-/admin/users/%d/edit", bot.ID)), http.StatusOK).Body)
		assert.Empty(t, doc.Find("#login_type").Nodes)
		assert.Empty(t, doc.Find("#password").Nodes)
		doc = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/-/admin/users/%d", bot.ID)), http.StatusOK).Body)
		assert.NotEmpty(t, doc.Find(`form[action$="/access_tokens"]`).Nodes)
	})

	t.Run("EditWithoutAuthSource", func(t *testing.T) {
		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/-/admin/users/%d/edit", bot.ID), map[string]string{
			"user_name": "bot-user",
			"email":     "bot-user@example.com",
			"full_name": "Bot User",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		bot = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: bot.ID})
		assert.Equal(t, "Bot User", bot.FullName)
		assert.True(t, bot.IsTypeBot())
		assert.Empty(t, bot.Passwd)
	})

	t.Run("TokenScope", func(t *testing.T) {
		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		tokenURL := fmt.Sprintf("/-/admin/users/%d/access_tokens", bot.ID)

		// a bot can never be a site administrator, so an admin-scoped token must be refused
		session.MakeRequest(t, NewRequestWithValues(t, "POST", tokenURL, map[string]string{
			"name":        "admin-scoped",
			"scope-admin": "write:admin",
		}), http.StatusSeeOther)
		assert.Equal(t, 0, unittest.GetCount(t, &auth_model.AccessToken{UID: bot.ID}))

		session.MakeRequest(t, NewRequestWithValues(t, "POST", tokenURL, map[string]string{
			"name":             "ci",
			"scope-repository": "write:repository",
		}), http.StatusSeeOther)
		assert.Equal(t, 1, unittest.GetCount(t, &auth_model.AccessToken{UID: bot.ID}))

		// tokens of non-bot accounts are managed by the account itself, not here
		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/users/2/access_tokens", map[string]string{
			"name":             "not-a-bot",
			"scope-repository": "write:repository",
		}), http.StatusSeeOther)
		unittest.AssertNotExistsBean(t, &auth_model.AccessToken{UID: 2, Name: "not-a-bot"})

		token := unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{UID: bot.ID, Name: "ci"})
		session.MakeRequest(t, NewRequestWithValues(t, "POST", tokenURL+"/delete", map[string]string{
			"id": strconv.FormatInt(token.ID, 10),
		}), http.StatusOK)
		assert.Equal(t, 0, unittest.GetCount(t, &auth_model.AccessToken{UID: bot.ID}))
	})

	t.Run("APIRejectsAuthSource", func(t *testing.T) {
		bot := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "bot-user"})
		for _, body := range []map[string]any{{"login_name": "cn=bot"}, {"source_id": 1}} {
			req := NewRequestWithJSON(t, "PATCH", "/api/v1/admin/users/"+bot.Name, body).AddBasicAuth("user1")
			MakeRequest(t, req, http.StatusBadRequest)
		}

		bot = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: bot.ID})
		assert.True(t, bot.IsLocal())
		assert.Empty(t, bot.LoginName)
	})

	t.Run("EditKeepsLoginNameWithoutLoginType", func(t *testing.T) {
		// the bot edit form omits login_type; that must not clear the login name of other accounts
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		require.NotEmpty(t, user2.LoginName)
		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/users/2/edit", map[string]string{
			"user_name": user2.Name,
			"email":     user2.Email,
		}), http.StatusSeeOther)
		assert.Equal(t, user2.LoginName, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).LoginName)
	})

	t.Run("ConvertType", func(t *testing.T) {
		convert := func(userID int64, userType string) {
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/-/admin/users/%d/convert_type", userID), map[string]string{"user_type": userType})
			session.MakeRequest(t, req, http.StatusSeeOther)
		}

		convert(4, "bot")
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		assert.True(t, user4.IsTypeBot())
		assert.Empty(t, user4.Passwd)

		convert(4, "individual")
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).IsIndividual())

		// an admin must not convert their own account, nor turn another site administrator into a bot
		convert(1, "bot")
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).IsIndividual())

		// bot accounts are non-interactive, so they cannot be impersonated
		convert(4, "bot")
		session.MakeRequest(t, NewRequest(t, "POST", "/-/admin/users/4/impersonate"), http.StatusBadRequest)
		convert(4, "individual")

		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/-/admin/users/99999/convert_type",
			map[string]string{"user_type": "bot"}), http.StatusNotFound)
	})
}
