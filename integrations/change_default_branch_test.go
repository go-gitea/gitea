// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestChangeDefaultBranch(t *testing.T) {
	prepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name, "password")
	branchesURL := fmt.Sprintf("/%s/%s/settings/branches", owner.Name, repo.Name)

	req := NewRequest(t, "GET", branchesURL)
	resp := session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)

	req = NewRequestBody(t, "POST", branchesURL,
		bytes.NewBufferString(url.Values{
			"_csrf":  []string{doc.GetInputValueByName("_csrf")},
			"action": []string{"default_branch"},
			"branch": []string{"DefaultBranch"},
		}.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)

	req = NewRequest(t, "GET", branchesURL)
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
	doc, err = NewHtmlParser(resp.Body)
	assert.NoError(t, err)

	req = NewRequestBody(t, "POST", branchesURL,
		bytes.NewBufferString(url.Values{
			"_csrf":  []string{doc.GetInputValueByName("_csrf")},
			"action": []string{"default_branch"},
			"branch": []string{"does_not_exist"},
		}.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = session.MakeRequest(t, req)
	assert.EqualValues(t, http.StatusNotFound, resp.HeaderCode)
}
