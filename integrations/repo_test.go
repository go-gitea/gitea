// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewRepo(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	req = NewRequest(t, "GET", "/user3/repo3")
	resp = MakeRequest(req)
	assert.EqualValues(t, http.StatusNotFound, resp.HeaderCode)

	session := loginUser(t, "user1")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusNotFound, resp.HeaderCode)
}

func TestViewRepo2(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}

func TestViewRepo3(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user3")
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}
