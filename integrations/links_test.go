// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"
)

func TestLinksNoLogin(t *testing.T) {
	prepareTestEnv(t)

	var links = []string{
		"/explore/repos",
		"/explore/repos?q=test&tab=",
		"/explore/users",
		"/explore/users?q=test&tab=",
		"/explore/organizations",
		"/explore/organizations?q=test&tab=",
		"/",
		"/user/sign_up",
		"/user/login",
		"/user/forgot_password",
		"/swagger",
	}

	for _, link := range links {
		req := NewRequest(t, "GET", link)
		MakeRequest(t, req, http.StatusOK)
	}
}
