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

func TestViewRepo(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	req, err := http.NewRequest("GET", "/user1/repo1", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}

func TestViewUser(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	req, err := http.NewRequest("GET", "/user1", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
}
