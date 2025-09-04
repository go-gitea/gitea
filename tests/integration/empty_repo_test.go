// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAPINewFile(t *testing.T, session *TestSession, user, repo, branch, treePath, content string) {
	url := fmt.Sprintf("/%s/%s/_new/%s", user, repo, branch)
	req := NewRequestWithValues(t, "POST", url, map[string]string{
		"_csrf":         GetUserCSRFToken(t, session),
		"commit_choice": "direct",
		"tree_path":     treePath,
		"content":       content,
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.NotEmpty(t, test.RedirectURL(resp))
}

func TestEmptyRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	subPaths := []string{
		"commits/master",
		"raw/foo",
		"commit/1ae57b34ccf7e18373",
		"graph",
	}
	emptyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 6})
	assert.True(t, emptyRepo.IsEmpty)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: emptyRepo.OwnerID})
	for _, subPath := range subPaths {
		req := NewRequestf(t, "GET", "/%s/%s/%s", owner.Name, emptyRepo.Name, subPath)
		MakeRequest(t, req, http.StatusNotFound)
	}
}

func TestEmptyRepoAddFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user30")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// test web page
	req := NewRequest(t, "GET", "/user30/empty")
	resp := session.MakeRequest(t, req, http.StatusOK)
	bodyString := resp.Body.String()
	assert.Contains(t, bodyString, "empty-repo-guide")
	assert.True(t, test.IsNormalPageCompleted(bodyString))

	// test api
	req = NewRequest(t, "GET", "/api/v1/repos/user30/empty/raw/main/README.md").AddTokenAuth(token)
	session.MakeRequest(t, req, http.StatusNotFound)

	// test feed
	req = NewRequest(t, "GET", "/user30/empty/rss/branch/main/README.md").AddTokenAuth(token).SetHeader("Accept", "application/rss+xml")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "</rss>")

	// create a new file
	req = NewRequest(t, "GET", "/user30/empty/_new/"+setting.Repository.DefaultBranch)
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body).Find(`input[name="commit_choice"]`)
	assert.Empty(t, doc.AttrOr("checked", "_no_"))
	req = NewRequestWithValues(t, "POST", "/user30/empty/_new/"+setting.Repository.DefaultBranch, map[string]string{
		"_csrf":         GetUserCSRFToken(t, session),
		"commit_choice": "direct",
		"tree_path":     "test-file.md",
		"content":       "newly-added-test-file",
	})

	resp = session.MakeRequest(t, req, http.StatusOK)
	redirect := test.RedirectURL(resp)
	assert.Equal(t, "/user30/empty/src/branch/"+setting.Repository.DefaultBranch+"/test-file.md", redirect)

	req = NewRequest(t, "GET", redirect)
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "newly-added-test-file")

	// the repo is not empty anymore
	req = NewRequest(t, "GET", "/user30/empty")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "test-file.md")

	// if the repo is in incorrect state, it should be able to self-heal (recover to correct state)
	testEmptyOrBrokenRecover := func(t *testing.T, isEmpty, isBroken bool) {
		user30EmptyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: 30, Name: "empty"})
		user30EmptyRepo.IsEmpty = isEmpty
		user30EmptyRepo.Status = util.Iif(isBroken, repo_model.RepositoryBroken, repo_model.RepositoryReady)
		user30EmptyRepo.DefaultBranch = "no-such"
		_, err := db.GetEngine(t.Context()).ID(user30EmptyRepo.ID).Cols("is_empty", "status", "default_branch").Update(user30EmptyRepo)
		require.NoError(t, err)
		user30EmptyRepo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: 30, Name: "empty"})
		assert.Equal(t, isEmpty, user30EmptyRepo.IsEmpty)
		assert.Equal(t, isBroken, user30EmptyRepo.Status == repo_model.RepositoryBroken)

		req = NewRequest(t, "GET", "/user30/empty")
		resp = session.MakeRequest(t, req, http.StatusSeeOther)
		redirect = test.RedirectURL(resp)
		assert.Equal(t, "/user30/empty", redirect)

		req = NewRequest(t, "GET", "/user30/empty")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), "test-file.md")
	}
	testEmptyOrBrokenRecover(t, true, false)
	testEmptyOrBrokenRecover(t, false, true)
	testEmptyOrBrokenRecover(t, true, true)
}

func TestEmptyRepoUploadFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user30")
	req := NewRequest(t, "GET", "/user30/empty/_new/"+setting.Repository.DefaultBranch)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body).Find(`input[name="commit_choice"]`)
	assert.Empty(t, doc.AttrOr("checked", "_no_"))

	body := &bytes.Buffer{}
	mpForm := multipart.NewWriter(body)
	_ = mpForm.WriteField("_csrf", GetUserCSRFToken(t, session))
	file, _ := mpForm.CreateFormFile("file", "uploaded-file.txt")
	_, _ = io.Copy(file, strings.NewReader("newly-uploaded-test-file"))
	_ = mpForm.Close()

	req = NewRequestWithBody(t, "POST", "/user30/empty/upload-file", body)
	req.Header.Add("Content-Type", mpForm.FormDataContentType())
	resp = session.MakeRequest(t, req, http.StatusOK)
	respMap := map[string]string{}
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respMap))

	req = NewRequestWithValues(t, "POST", "/user30/empty/_upload/"+setting.Repository.DefaultBranch, map[string]string{
		"_csrf":         GetUserCSRFToken(t, session),
		"commit_choice": "direct",
		"files":         respMap["uuid"],
		"tree_path":     "",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	redirect := test.RedirectURL(resp)
	assert.Equal(t, "/user30/empty/src/branch/"+setting.Repository.DefaultBranch, redirect)

	req = NewRequest(t, "GET", redirect)
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "uploaded-file.txt")
}

func TestEmptyRepoAddFileByAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user30")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user30/empty/contents/new-file.txt", &api.CreateFileOptions{
		FileOptions: api.FileOptions{
			NewBranchName: "new_branch",
			Message:       "init",
		},
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("newly-added-api-file")),
	}).AddTokenAuth(token)

	resp := MakeRequest(t, req, http.StatusCreated)
	var fileResponse api.FileResponse
	DecodeJSON(t, resp, &fileResponse)
	expectedHTMLURL := setting.AppURL + "user30/empty/src/branch/new_branch/new-file.txt"
	assert.Equal(t, expectedHTMLURL, *fileResponse.Content.HTMLURL)

	req = NewRequest(t, "GET", "/user30/empty/src/branch/new_branch/new-file.txt")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "newly-added-api-file")

	req = NewRequest(t, "GET", "/api/v1/repos/user30/empty").
		AddTokenAuth(token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiRepo api.Repository
	DecodeJSON(t, resp, &apiRepo)
	assert.Equal(t, "new_branch", apiRepo.DefaultBranch)
}
