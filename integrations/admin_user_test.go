// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strconv"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestAdminViewUsers(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/admin/users")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/admin/users")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAdminViewUser(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/admin/users/1")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/admin/users/1")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAdminEditUser(t *testing.T) {
	defer prepareTestEnv(t)()

	testSuccessfullEdit(t, models.User{ID: 2, Name: "newusername", LoginName: "otherlogin", Email: "new@e-mail.gitea"})
}

func testSuccessfullEdit(t *testing.T, formData models.User) {
	makeRequest(t, formData, http.StatusFound)
}

func makeRequest(t *testing.T, formData models.User, headerCode int) {
	session := loginUser(t, "user1")
	csrf := GetCSRF(t, session, "/admin/users/"+strconv.Itoa(int(formData.ID)))
	req := NewRequestWithValues(t, "POST", "/admin/users/"+strconv.Itoa(int(formData.ID)), map[string]string{
		"_csrf":      csrf,
		"user_name":  formData.Name,
		"login_name": formData.LoginName,
		"login_type": "0-0",
		"email":      formData.Email,
	})

	session.MakeRequest(t, req, headerCode)
	user := unittest.AssertExistsAndLoadBean(t, &models.User{ID: formData.ID}).(*models.User)
	assert.Equal(t, formData.Name, user.Name)
	assert.Equal(t, formData.LoginName, user.LoginName)
	assert.Equal(t, formData.Email, user.Email)
}

func TestAdminDeleteUser(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user1")

	csrf := GetCSRF(t, session, "/admin/users/8")
	req := NewRequestWithValues(t, "POST", "/admin/users/8/delete", map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusOK)

	assertUserDeleted(t, 8)
	unittest.CheckConsistencyFor(t, &models.User{})
}
