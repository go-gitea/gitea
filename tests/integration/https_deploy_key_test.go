// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addTestHTTPSDeployKey is a small helper that creates an HTTPS deploy key in
// the test database and returns the plaintext token. It assumes the fixtures
// already hold a repo with the given ID.
func addTestHTTPSDeployKey(t *testing.T, repoID int64, name string, readOnly bool) string {
	t.Helper()
	_, token, err := asymkey_model.AddHTTPSDeployKey(t.Context(), repoID, name, readOnly)
	require.NoError(t, err)
	return token
}

func TestHTTPSDeployKeyClone_Read(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	token := addTestHTTPSDeployKey(t, repo.ID, "https-clone-read", true)

	req := NewRequest(t, "GET", "/"+repo.FullName()+"/info/refs?service=git-upload-pack")
	req.Request.SetBasicAuth("x-deploy-token", token)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "service=git-upload-pack")
}

func TestHTTPSDeployKeyPush_ReadOnlyDenied(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	token := addTestHTTPSDeployKey(t, repo.ID, "https-push-denied", true)

	req := NewRequest(t, "GET", "/"+repo.FullName()+"/info/refs?service=git-receive-pack")
	req.Request.SetBasicAuth("x-deploy-token", token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestHTTPSDeployKeyPush_WriteAllowed(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	token := addTestHTTPSDeployKey(t, repo.ID, "https-push-allowed", false)

	// `GET info/refs?service=git-receive-pack` is the same auth path a
	// real push would hit first; a 200 means the server would accept the
	// push as far as auth is concerned.
	req := NewRequest(t, "GET", "/"+repo.FullName()+"/info/refs?service=git-receive-pack")
	req.Request.SetBasicAuth("x-deploy-token", token)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "service=git-receive-pack")
}

func TestHTTPSDeployKeyCrossRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	token := addTestHTTPSDeployKey(t, repo1.ID, "https-cross-repo", false)

	req := NewRequest(t, "GET", "/"+repo2.FullName()+"/info/refs?service=git-upload-pack")
	req.Request.SetBasicAuth("x-deploy-token", token)
	// A cross-repo token should be treated exactly like a missing credential
	// for repo2 — the server must not leak whether the token is otherwise
	// valid.
	MakeRequest(t, req, http.StatusNotFound)
}

func TestHTTPSDeployKeyInvalidToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// 40 hex chars that do not correspond to any stored token: the server
	// must fall through to Basic auth which then rejects it.
	req := NewRequest(t, "GET", "/"+repo.FullName()+"/info/refs?service=git-upload-pack")
	req.Request.SetBasicAuth("x-deploy-token", "0123456789abcdef0123456789abcdef01234567")
	MakeRequest(t, req, http.StatusUnauthorized)
}

// TestHTTPSDeployKeyScopedToGitHTTP asserts that a deploy token does NOT
// authenticate on non-git Basic-auth endpoints. Historically the token check
// lived inside auth.Basic.VerifyAuthToken, which made the token behave like a
// full owner-scoped PAT on the REST API, attachments, feeds, etc.
func TestHTTPSDeployKeyScopedToGitHTTP(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	token := addTestHTTPSDeployKey(t, repo.ID, "scope-guard", false)

	// The token works on a git-HTTP path…
	gitReq := NewRequest(t, "GET", "/"+repo.FullName()+"/info/refs?service=git-upload-pack")
	gitReq.Request.SetBasicAuth("x-deploy-token", token)
	MakeRequest(t, gitReq, http.StatusOK)

	// …but the same token on the REST API must NOT authenticate. The API
	// layer only knows user/PAT credentials, so the request is rejected as
	// an invalid credential (401) rather than silently admitted as the
	// repo owner.
	apiReq := NewRequest(t, "GET", "/api/v1/repos/"+repo.FullName())
	apiReq.Request.SetBasicAuth("x-deploy-token", token)
	MakeRequest(t, apiReq, http.StatusUnauthorized)
}

// TestHTTPSDeployKeyRespectsDisabledWikiUnit asserts that a deploy token
// cannot reach the wiki of a repository whose wiki unit has been disabled.
// Before the fix the deploy-token branch in httpBase short-circuited before
// the unit-enablement check that runs for password / PAT paths.
func TestHTTPSDeployKeyRespectsDisabledWikiUnit(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo, nil, []unit.Type{unit.TypeWiki}))

	token := addTestHTTPSDeployKey(t, repo.ID, "wiki-guard", false)

	wikiReq := NewRequest(t, "GET", "/"+repo.FullName()+".wiki/info/refs?service=git-upload-pack")
	wikiReq.Request.SetBasicAuth("x-deploy-token", token)
	MakeRequest(t, wikiReq, http.StatusForbidden)
}
