// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
)

func TestDeleteUser(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user1")

	csrf := GetCSRF(t, session, "/admin/users/8")
	req := NewRequestWithValues(t, "POST", "/admin/users/8/delete", map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusOK)

	models.AssertNotExistsBean(t, &models.User{ID: 8})
	models.CheckConsistencyFor(t, &models.User{})
}
