// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"
)

func TestEmptyRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	subpaths := []string{
		"commits/master",
		"raw/foo",
		"commit/1ae57b34ccf7e18373",
		"graph",
	}
	emptyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{}, unittest.Cond("is_empty = ?", true))
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: emptyRepo.OwnerID})
	for _, subpath := range subpaths {
		req := NewRequestf(t, "GET", "/%s/%s/%s", owner.Name, emptyRepo.Name, subpath)
		MakeRequest(t, req, http.StatusNotFound)
	}
}
