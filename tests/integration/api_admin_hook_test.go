// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/tests"
)

func TestGetNotExistHook(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		defer tests.PrepareTestEnv(t)()

		session := loginUser(t, "user1")
		req := NewRequest(t, "GET", "/api/v1/admin/hooks/1234")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestDeleteNotExistHook(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		defer tests.PrepareTestEnv(t)()

		session := loginUser(t, "user1")
		req := NewRequest(t, "DELETE", "/api/v1/admin/hooks/1234")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}
