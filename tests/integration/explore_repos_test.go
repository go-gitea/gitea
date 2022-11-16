// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"
)

func TestExploreRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/explore/repos")
	MakeRequest(t, req, http.StatusOK)
}
