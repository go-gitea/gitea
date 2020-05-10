// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
)

func TestChangeDefaultBranch(t *testing.T) {
	defer prepareTestEnv(t)()
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	branchesURL := fmt.Sprintf("/%s/%s/settings/branches", owner.Name, repo.Name)

	csrf := GetCSRF(t, session, branchesURL)
	req := NewRequestWithValues(t, "POST", branchesURL, map[string]string{
		"_csrf":  csrf,
		"action": "default_branch",
		"branch": "DefaultBranch",
	})
	session.MakeRequest(t, req, http.StatusFound)

	csrf = GetCSRF(t, session, branchesURL)
	req = NewRequestWithValues(t, "POST", branchesURL, map[string]string{
		"_csrf":  csrf,
		"action": "default_branch",
		"branch": "does_not_exist",
	})
	session.MakeRequest(t, req, http.StatusNotFound)
}
