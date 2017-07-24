// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	api "code.gitea.io/sdk/gitea"
)

func TestViewDeployKeysNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/keys")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateDeployKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/keys", api.CreateKeyOption{
		Title: "title",
		Key:   "key",
	})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestGetDeployKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestDeleteDeployKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "DELETE", "/api/v1/repos/user2/repo1/keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}
