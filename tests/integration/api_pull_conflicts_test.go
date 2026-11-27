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
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIPullRequestConflicts(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		const repoURL = "/api/v1/repos/user2/repo1"
		token := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)

		createBranch := func(name string) {
			req := NewRequestWithJSON(t, "POST", repoURL+"/branches", &api.CreateBranchRepoOption{BranchName: name, OldRefName: "master"}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}
		writeFile := func(branch, path, content string) {
			req := NewRequestWithJSON(t, "POST", repoURL+"/contents", &api.ChangeFilesOptions{
				FileOptions: api.FileOptions{BranchName: branch},
				Files:       []*api.ChangeFileOperation{{Operation: "upload", Path: path, ContentBase64: base64.StdEncoding.EncodeToString([]byte(content))}},
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}
		newPull := func(base, head string) int64 {
			req := NewRequestWithJSON(t, "POST", repoURL+"/pulls", &api.CreatePullRequestOption{Base: base, Head: head, Title: head}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			return DecodeJSON(t, resp, &api.PullRequest{}).Index
		}
		getConflicts := func(index int64, status int) *httptest.ResponseRecorder {
			return MakeRequest(t, NewRequestf(t, "GET", "%s/pulls/%d/conflicts", repoURL, index).AddTokenAuth(token), status)
		}

		createBranch("cf-base")
		createBranch("cf-head")
		createBranch("cf-clean")
		writeFile("cf-base", "both.txt", "base side\n")
		writeFile("cf-head", "both.txt", "head side\n")
		writeFile("cf-head", "only-head.txt", "no conflict\n")
		writeFile("cf-clean", "clean.txt", "no conflict\n")
		conflictedPull := newPull("cf-base", "cf-head")
		cleanPull := newPull("cf-base", "cf-clean")

		if !git.DefaultFeatures().SupportGitMergeTree {
			getConflicts(conflictedPull, http.StatusNotImplemented)
			return
		}

		t.Run("Conflicted", func(t *testing.T) {
			result := DecodeJSON(t, getConflicts(conflictedPull, http.StatusOK), &api.PullRequestConflicts{})
			assert.True(t, result.Conflicted)
			assert.False(t, result.Truncated)
			require.Len(t, result.Files, 1)
			file := result.Files[0]
			assert.Equal(t, "both.txt", file.Path)
			assert.Equal(t, "both_added", file.Kind)
			assert.Nil(t, file.Base)
			require.NotNil(t, file.Ours)
			require.NotNil(t, file.Theirs)
			assert.NotEqual(t, file.Ours.SHA, file.Theirs.SHA)
			require.NotNil(t, file.Content)
			assert.Contains(t, *file.Content, "<<<<<<<")
			assert.Contains(t, *file.Content, "base side")
			assert.Contains(t, *file.Content, "head side")
		})

		t.Run("Clean", func(t *testing.T) {
			result := DecodeJSON(t, getConflicts(cleanPull, http.StatusOK), &api.PullRequestConflicts{})
			assert.False(t, result.Conflicted)
			assert.Empty(t, result.Files)
		})

		t.Run("Errors", func(t *testing.T) {
			getConflicts(99999, http.StatusNotFound)
			MakeRequest(t, NewRequestf(t, "GET", "/api/v1/repos/user2/repo16/pulls/1/conflicts"), http.StatusNotFound)
		})
	})
}
