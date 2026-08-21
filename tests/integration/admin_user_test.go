// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

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
