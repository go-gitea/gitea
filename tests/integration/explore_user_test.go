// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestExploreUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/explore/users")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<a class="active item" href="/explore/users?sort=newest&q=">Newest</a>`)

	req = NewRequest(t, "GET", "/explore/users?sort=oldest")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<a class="active item" href="/explore/users?sort=oldest&q=">Oldest</a>`)

	req = NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<a class="active item" href="/explore/users?sort=alphabetically&q=">Alphabetically</a>`)

	req = NewRequest(t, "GET", "/explore/users?sort=reversealphabetically")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<a class="active item" href="/explore/users?sort=reversealphabetically&q=">Reverse alphabetically</a>`)

	// these sort orders shouldn't be supported, to avoid leaking user activity
	cases := []string{
		"/explore/users?sort=lastlogin",
		"/explore/users?sort=reverselastlogin",
		"/explore/users?sort=leastupdate",
		"/explore/users?sort=reverseleastupdate",
	}
	for _, c := range cases {
		req = NewRequest(t, "GET", c).SetHeader("Accept", "text/html")
		resp = MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), `<title>Page Not Found`)
	}
}
