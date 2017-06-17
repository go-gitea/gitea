// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestChangeDefaultBranch(t *testing.T) {
	prepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	branchesURL := fmt.Sprintf("/%s/%s/settings/branches", owner.Name, repo.Name)

	req := NewRequest(t, "GET", branchesURL)
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	doc := NewHtmlParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", branchesURL, map[string]string{
		"_csrf":  doc.GetCSRF(),
		"action": "default_branch",
		"branch": "DefaultBranch",
	})
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)

	req = NewRequest(t, "GET", branchesURL)
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	doc = NewHtmlParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", branchesURL, map[string]string{
		"_csrf":  doc.GetInputValueByName("_csrf"),
		"action": "default_branch",
		"branch": "does_not_exist",
	})
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusNotFound, resp.HeaderCode)
}
