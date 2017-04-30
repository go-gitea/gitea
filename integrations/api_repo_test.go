// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserReposNotLogin(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	req, err := http.NewRequest("GET", "/api/v1/users/user2/repos", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}

func TestAPISearchRepoNotLogin(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	req, err := http.NewRequest("GET", "/api/v1/repos/search?q=Test", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}
