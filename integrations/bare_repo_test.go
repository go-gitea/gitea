// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
)

func TestBareRepo(t *testing.T) {
	prepareTestEnv(t)
	subpaths := []string{
		"commits/master",
		"raw/foo",
		"commit/1ae57b34ccf7e18373",
		"graph",
	}
	bareRepo := models.AssertExistsAndLoadBean(t, &models.Repository{}, models.Cond("is_bare = ?", true)).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: bareRepo.OwnerID}).(*models.User)
	for _, subpath := range subpaths {
		req := NewRequestf(t, "GET", "/%s/%s/%s", owner.Name, bareRepo.Name, subpath)
		MakeRequest(t, req, http.StatusNotFound)
	}
}
