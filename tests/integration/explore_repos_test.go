// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestExploreRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/explore/repos?q=TheKeyword&topic=1&language=TheLang")
	resp := MakeRequest(t, req, http.StatusOK)
	respStr := resp.Body.String()

	assert.Contains(t, respStr, `<input type="hidden" name="topic" value="true">`)
	assert.Contains(t, respStr, `<input type="hidden" name="language" value="TheLang">`)
	assert.Contains(t, respStr, `<input type="search" name="q" value="TheKeyword"`)
}
