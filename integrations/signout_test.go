// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"
)

func TestSignOut(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "POST", "/user/logout")
	session.MakeRequest(t, req, http.StatusFound)

	// try to view a private repo, should fail
	req = NewRequest(t, "GET", "/user2/repo2/")
	session.MakeRequest(t, req, http.StatusNotFound)

	// invalidate cached cookies for user2, for subsequent tests
	delete(loginSessionCache, "user2")
}
