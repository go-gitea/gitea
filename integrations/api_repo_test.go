// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"
)

func TestAPIUserReposNotLogin(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/api/v1/users/user2/repos")
	MakeRequest(t, req, http.StatusOK)
}

func TestAPISearchRepoNotLogin(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/api/v1/repos/search?q=Test")
	MakeRequest(t, req, http.StatusOK)
}
