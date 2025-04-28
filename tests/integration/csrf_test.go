// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCsrfProtection(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// test web form csrf via form
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"_csrf": "fake_csrf",
	})
	resp := session.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Invalid CSRF token")

	// test web form csrf via header. TODO: should use an UI api to test
	req = NewRequest(t, "POST", "/user/settings")
	req.Header.Add("X-Csrf-Token", "fake_csrf")
	resp = session.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Invalid CSRF token")
}
