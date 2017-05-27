// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoLoginViewIssues(t *testing.T) {
	prepareTestEnv(t)

	req, err := http.NewRequest("GET", "/user2/repo1/issues", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}

func TestNoLoginViewIssue(t *testing.T) {
	prepareTestEnv(t)

	req, err := http.NewRequest("GET", "/user2/repo1/issues/1", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}
