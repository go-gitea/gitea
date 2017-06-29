// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"
)

func TestViewRepo(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user3/repo3")
	MakeRequest(t, req, http.StatusNotFound)

	session := loginUser(t, "user1")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestViewRepo2(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user2")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo3(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user3")
	session.MakeRequest(t, req, http.StatusOK)
}
