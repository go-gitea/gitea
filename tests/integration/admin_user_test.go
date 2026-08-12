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
	"gitea.dev/services/auth/source/ldap"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
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

func TestAdminViewUsersFilterAuthSource(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	source := &auth_model.Source{Type: auth_model.LDAP, Name: "test-user-list-filter", IsActive: true, Cfg: &ldap.Source{}}
	require.NoError(t, auth_model.CreateSource(t.Context(), source))

	user2 := &user_model.User{ID: 2, LoginType: auth_model.LDAP, LoginSource: source.ID}
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user2, "login_type", "login_source"))

	session := loginUser(t, "user1")
	listUsers := func(query string) (*HTMLDoc, []string) {
		req := NewRequest(t, "GET", "/-/admin/users?"+query)
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		return doc, doc.Find("table tbody tr td:nth-child(2) a").Map(func(_ int, s *goquery.Selection) string {
			return s.Text()
		})
	}

	doc, users := listUsers("source_id=") // the "All" option submits an empty value
	AssertHTMLElement(t, doc, `input[name="source_id"][value=""][checked]`, true)
	assert.Subset(t, users, []string{"user1", "user2"})

	doc, users = listUsers(fmt.Sprintf("source_id=%d", source.ID)) // the "test-user-list-filter" LDAP source
	AssertHTMLElement(t, doc, fmt.Sprintf(`input[name="source_id"][value="%d"][checked]`, source.ID), true)
	assert.Equal(t, []string{"user2"}, users)
	assert.Equal(t, source.Name, doc.Find("table tbody tr td:nth-child(4)").Text())

	_, users = listUsers("source_id=0") // 0 means the "Local" source
	assert.Contains(t, users, "user1")
	assert.NotContains(t, users, "user2")
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

	session := loginUser(t, "user1")
	currentUsername := func(t *testing.T) string {
		t.Helper()
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		return doc.Find("[data-signed-in-username]").AttrOr("data-signed-in-username", "")
	}

	// user1 is admin, can visit admin pages
	assert.Equal(t, "user1", currentUsername(t))
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2"), http.StatusOK)

	// impersonate to user2, user2 can't visit admin pages
	session.MakeRequest(t, NewRequest(t, "POST", "/-/admin/users/2/impersonate"), http.StatusOK)
	assert.Equal(t, "user2", currentUsername(t))
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2"), http.StatusForbidden)

	// exit impersonation, current user is user1(admin) again
	session.MakeRequest(t, NewRequest(t, "GET", "/user/logout"), http.StatusSeeOther)
	assert.Equal(t, "user1", currentUsername(t))
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2"), http.StatusOK)

	// completely logout
	session.MakeRequest(t, NewRequest(t, "GET", "/user/logout"), http.StatusSeeOther)
	assert.Equal(t, "", currentUsername(t))
}
