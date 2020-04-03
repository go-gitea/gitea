// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package integrations

import (
	"net/http"
	"testing"
)

func TestPullView_ReviewerMissed(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", "/pulls")
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user2/repo1/pulls/3")
	session.MakeRequest(t, req, http.StatusOK)
}
