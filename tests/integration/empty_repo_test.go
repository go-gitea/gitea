// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestEmptyRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	subPaths := []string{
		"commits/master",
		"raw/foo",
		"commit/1ae57b34ccf7e18373",
		"graph",
	}
	emptyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	assert.True(t, emptyRepo.IsEmpty)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: emptyRepo.OwnerID})
	for _, subPath := range subPaths {
		req := NewRequestf(t, "GET", "/%s/%s/%s", owner.Name, emptyRepo.Name, subPath)
		MakeRequest(t, req, http.StatusNotFound)
	}
}

func TestEmptyRepoAddFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	err := user_model.UpdateUserCols(db.DefaultContext, &user_model.User{ID: 30, ProhibitLogin: false}, "prohibit_login")
	assert.NoError(t, err)

	session := loginUser(t, "user30")
	req := NewRequest(t, "GET", "/user30/empty/_new/"+setting.Repository.DefaultBranch)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body).Find(`input[name="commit_choice"]`)
	assert.Empty(t, doc.AttrOr("checked", "_no_"))
	req = NewRequestWithValues(t, "POST", "/user30/empty/_new/"+setting.Repository.DefaultBranch, map[string]string{
		"_csrf":         GetCSRF(t, session, "/user/settings"),
		"commit_choice": "direct",
		"tree_path":     "test-file.md",
		"content":       "newly-added-test-file",
	})

	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	redirect := test.RedirectURL(resp)
	assert.Equal(t, "/user30/empty/src/branch/"+setting.Repository.DefaultBranch+"/test-file.md", redirect)

	req = NewRequest(t, "GET", redirect)
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "newly-added-test-file")
}

func TestEmptyRepoUploadFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	err := user_model.UpdateUserCols(db.DefaultContext, &user_model.User{ID: 30, ProhibitLogin: false}, "prohibit_login")
	assert.NoError(t, err)

	session := loginUser(t, "user30")
	req := NewRequest(t, "GET", "/user30/empty/_new/"+setting.Repository.DefaultBranch)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body).Find(`input[name="commit_choice"]`)
	assert.Empty(t, doc.AttrOr("checked", "_no_"))

	body := &bytes.Buffer{}
	mpForm := multipart.NewWriter(body)
	_ = mpForm.WriteField("_csrf", GetCSRF(t, session, "/user/settings"))
	file, _ := mpForm.CreateFormFile("file", "uploaded-file.txt")
	_, _ = io.Copy(file, bytes.NewBufferString("newly-uploaded-test-file"))
	_ = mpForm.Close()

	req = NewRequestWithBody(t, "POST", "/user30/empty/upload-file", body)
	req.Header.Add("Content-Type", mpForm.FormDataContentType())
	resp = session.MakeRequest(t, req, http.StatusOK)
	respMap := map[string]string{}
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respMap))

	req = NewRequestWithValues(t, "POST", "/user30/empty/_upload/"+setting.Repository.DefaultBranch, map[string]string{
		"_csrf":         GetCSRF(t, session, "/user/settings"),
		"commit_choice": "direct",
		"files":         respMap["uuid"],
		"tree_path":     "",
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	redirect := test.RedirectURL(resp)
	assert.Equal(t, "/user30/empty/src/branch/"+setting.Repository.DefaultBranch+"/", redirect)

	req = NewRequest(t, "GET", redirect)
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "uploaded-file.txt")
}
