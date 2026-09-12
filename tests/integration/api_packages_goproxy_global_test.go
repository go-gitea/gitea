// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setGlobalGoProxyAppURL(t *testing.T) {
	t.Helper()

	oldAppURL, oldAppSubURL := setting.AppURL, setting.AppSubURL
	setting.AppURL = "http://gitea.example.com/"
	setting.AppSubURL = ""
	t.Cleanup(func() {
		setting.AppURL, setting.AppSubURL = oldAppURL, oldAppSubURL
	})
}

func prepareGoModuleRepo(t *testing.T, repo *repo_model.Repository, modulePath string) {
	t.Helper()

	gitBin, err := exec.LookPath("git")
	require.NoError(t, err)
	runGit := func(dir string, args ...string) {
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Gitea", "GIT_AUTHOR_EMAIL=gitea@example.com", "GIT_COMMITTER_NAME=Gitea", "GIT_COMMITTER_EMAIL=gitea@example.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	workTree := t.TempDir()
	repoPath := gitrepo.RepoLocalPath(repo)
	require.NoError(t, git.Clone(t.Context(), repoPath, workTree, git.CloneRepoOptions{Shared: true}))
	require.NoError(t, os.WriteFile(filepath.Join(workTree, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.27\n"), 0o644))
	runGit(workTree, "add", "go.mod")
	runGit(workTree, "commit", "-m", "add go.mod")
	runGit(workTree, "tag", "v1.0.0")
	runGit(repoPath, "fetch", workTree, "+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*")
}

func TestGlobalGoProxyUpstream(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	var listHits, modHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/github.com/foo/bar/@v/list":
			listHits.Add(1)
			w.Header().Set("Content-Type", "text/plain;charset=utf-8")
			_, _ = w.Write([]byte("v1.0.0\n"))
		case "/github.com/foo/bar/@v/v1.0.0.info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Version":"v1.0.0","Time":"2025-01-01T00:00:00Z"}`))
		case "/github.com/foo/bar/@latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Version":"v1.0.0","Time":"2025-01-01T00:00:00Z"}`))
		case "/github.com/foo/bar/@v/v1.0.0.mod":
			modHits.Add(1)
			w.Header().Set("Content-Type", "text/plain;charset=utf-8")
			_, _ = w.Write([]byte("module github.com/foo/bar\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	oldGoProxyURL := setting.Packages.GoProxyURL
	setting.Packages.GoProxyURL = upstream.URL
	defer func() {
		setting.Packages.GoProxyURL = oldGoProxyURL
	}()

	req := NewRequest(t, http.MethodGet, "/api/packages/go/github.com/foo/bar/@v/list")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "v1.0.0\n", resp.Body.String())
	assert.Equal(t, int64(1), listHits.Load())

	req = NewRequest(t, http.MethodGet, "/api/packages/go/github.com/foo/bar/@v/v1.0.0.mod")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "module github.com/foo/bar\n", resp.Body.String())
	assert.Equal(t, int64(1), modHits.Load())

	// Immutable module files are cached after the first successful fetch.
	req = NewRequest(t, http.MethodGet, "/api/packages/go/github.com/foo/bar/@v/v1.0.0.mod")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "module github.com/foo/bar\n", resp.Body.String())
	assert.Equal(t, int64(1), modHits.Load())

	req = NewRequest(t, http.MethodGet, fmt.Sprintf("/api/packages/go/github.com/foo/bar/@v/%s.info", "v1.0.0"))
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `"Version":"v1.0.0"`)

	req = NewRequest(t, http.MethodGet, "/api/packages/go/github.com/foo/bar/@latest")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `"Version":"v1.0.0"`)
}

func TestGlobalGoProxyPipeFallback(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	var upstreamHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/github.com/foo/bar/@v/list" {
			upstreamHits.Add(1)
			_, _ = w.Write([]byte("v1.0.0\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failed", http.StatusInternalServerError)
	}))
	defer failing.Close()

	oldGoProxyURL := setting.Packages.GoProxyURL
	setting.Packages.GoProxyURL = failing.URL + "|" + upstream.URL
	defer func() {
		setting.Packages.GoProxyURL = oldGoProxyURL
	}()

	req := NewRequest(t, http.MethodGet, "/api/packages/go/github.com/foo/bar/@v/list")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "v1.0.0\n", resp.Body.String())
	assert.Equal(t, int64(1), upstreamHits.Load())
}

func TestGlobalGoProxyLocalRepository(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setGlobalGoProxyAppURL(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	prepareGoModuleRepo(t, repo, "gitea.example.com/user2/repo1")

	req := NewRequest(t, http.MethodGet, "/api/packages/go/gitea.example.com/user2/repo1/@v/list")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "v1.0.0\n", resp.Body.String())

	req = NewRequest(t, http.MethodGet, "/api/packages/go/gitea.example.com/user2/repo1/@v/v1.0.0.mod")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "module gitea.example.com/user2/repo1\n\ngo 1.27\n", resp.Body.String())

	req = NewRequest(t, http.MethodGet, "/api/packages/go/gitea.example.com/user2/repo1/@v/v1.0.0.zip")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "application/zip", resp.Header().Get("Content-Type"))
}

func TestGlobalGoProxyPrivateRepository(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setGlobalGoProxyAppURL(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
	prepareGoModuleRepo(t, repo, "gitea.example.com/user2/repo16")

	req := NewRequest(t, http.MethodGet, "/api/packages/go/gitea.example.com/user2/repo16/@v/list")
	MakeRequest(t, req, http.StatusNotFound)

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	req = NewRequest(t, http.MethodGet, "/api/packages/go/gitea.example.com/user2/repo16/@v/list").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "v1.0.0\n", resp.Body.String())
}
