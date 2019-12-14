// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloadByID(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo1/raw/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
}

func TestDownloadByIDMedia(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo1/media/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
}
