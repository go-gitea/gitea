// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPICherryPickAndRevert(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		token4 := getTokenForLoggedInUser(t, loginUser(t, "user4"), auth_model.AccessTokenScopeWriteRepository)
		const repoURL = "/api/v1/repos/user2/repo1"

		createBranch := func(name string) {
			req := NewRequestWithJSON(t, "POST", repoURL+"/branches", &api.CreateBranchRepoOption{BranchName: name, OldRefName: "master"}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}
		commitFile := func(branch, path, content string) string {
			req := NewRequestWithJSON(t, "POST", repoURL+"/contents/"+path, &api.CreateFileOptions{
				FileOptions:   api.FileOptions{BranchName: branch},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte(content)),
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			return DecodeJSON(t, resp, &api.FileResponse{}).Commit.SHA
		}
		fileContent := func(ref, path string) (string, int) {
			req := NewRequest(t, "GET", repoURL+"/raw/"+path+"?ref="+ref).AddTokenAuth(token)
			resp := session.MakeRequest(t, req, NoExpectedStatus)
			return resp.Body.String(), resp.Code
		}

		createBranch("cp-source")
		createBranch("cp-target")
		sha := commitFile("cp-source", "cp-file.txt", "from source\n")
		post := func(action, commitSHA string, opts api.CherryPickOptions, tok string, status int) *httptest.ResponseRecorder {
			req := NewRequestWithJSON(t, "POST", repoURL+"/commits/"+commitSHA+"/"+action, &opts)
			if tok != "" {
				req.AddTokenAuth(tok)
			}
			return MakeRequest(t, req, status)
		}

		t.Run("DryRun", func(t *testing.T) {
			post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-dry"}, DryRun: true}, token, http.StatusNoContent)
			MakeRequest(t, NewRequest(t, "GET", repoURL+"/branches/cp-dry").AddTokenAuth(token), http.StatusNotFound)
		})

		t.Run("CherryPickToNewBranch", func(t *testing.T) {
			resp := post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-picked"}}, token, http.StatusCreated)
			fileResponse := DecodeJSON(t, resp, &api.FileResponse{})
			assert.Equal(t, "Cherry-pick: "+sha+"\n", fileResponse.Commit.Message)
			content, _ := fileContent("cp-picked", "cp-file.txt")
			assert.Equal(t, "from source\n", content)
			_, code := fileContent("cp-target", "cp-file.txt")
			assert.Equal(t, http.StatusNotFound, code, "the base branch must stay untouched")
		})

		t.Run("Failures", func(t *testing.T) {
			// the new branch already exists
			post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-picked"}}, token, http.StatusUnprocessableEntity)
			// unknown commit
			post("cherry-pick", "0123456789abcdef0123456789abcdef01234567", api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-missing"}}, token, http.StatusNotFound)
			// not a commit id, and objects that are not commits
			post("cherry-pick", "master", api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-badsha"}}, token, http.StatusUnprocessableEntity)
			post("cherry-pick", "--help", api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-badsha"}}, token, http.StatusUnprocessableEntity)
			req := NewRequest(t, "GET", repoURL+"/git/commits/cp-source").AddTokenAuth(token)
			treeSHA := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.Commit{}).RepoCommit.Tree.SHA
			post("cherry-pick", treeSHA, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-badsha"}}, token, http.StatusNotFound)
			// the change is already on the branch
			post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-picked", NewBranchName: "cp-noop"}}, token, http.StatusConflict)
			// no write permission, and no token
			post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-denied"}}, token4, http.StatusForbidden)
			post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-denied"}}, "", http.StatusUnauthorized)
		})

		t.Run("Conflict", func(t *testing.T) {
			commitFile("cp-target", "cp-file.txt", "from target\n")
			for _, dryRun := range []bool{true, false} {
				resp := post("cherry-pick", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-target", NewBranchName: "cp-conflict"}, DryRun: dryRun}, token, http.StatusConflict)
				assert.Contains(t, resp.Body.String(), "cp-file.txt")
			}
			MakeRequest(t, NewRequest(t, "GET", repoURL+"/branches/cp-conflict").AddTokenAuth(token), http.StatusNotFound)
		})

		t.Run("Revert", func(t *testing.T) {
			resp := post("revert", sha, api.CherryPickOptions{FileOptions: api.FileOptions{BranchName: "cp-source"}}, token, http.StatusCreated)
			assert.Equal(t, "Revert: "+sha+"\n", DecodeJSON(t, resp, &api.FileResponse{}).Commit.Message)
			_, code := fileContent("cp-source", "cp-file.txt")
			assert.Equal(t, http.StatusNotFound, code)
		})
	})
}
