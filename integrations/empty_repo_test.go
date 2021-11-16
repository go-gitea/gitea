// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
)

func TestEmptyRepo(t *testing.T) {
	defer prepareTestEnv(t)()
	subpaths := []string{
		"commits/master",
		"raw/foo",
		"commit/1ae57b34ccf7e18373",
		"graph",
	}
	emptyRepo := unittest.AssertExistsAndLoadBean(t, &models.Repository{}, unittest.Cond("is_empty = ?", true)).(*models.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &models.User{ID: emptyRepo.OwnerID}).(*models.User)
	for _, subpath := range subpaths {
		req := NewRequestf(t, "GET", "/%s/%s/%s", owner.Name, emptyRepo.Name, subpath)
		MakeRequest(t, req, http.StatusNotFound)
	}
}
