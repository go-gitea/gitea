// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	api "code.gitea.io/sdk/gitea"
)

func TestViewOwnGPGKeysNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/user/gpg_keys")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestViewGPGKeysNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/users/user2/gpg_keys")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestCreateGPGKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_keys", api.CreateGPGKeyOption{
		ArmoredKey: "key",
	})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestGetGPGKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/api/v1/user/gpg_keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestDeleteGPGKeyNoLogin(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "DELETE", "/api/v1/user/gpg_keys/1")
	MakeRequest(t, req, http.StatusUnauthorized)
}
