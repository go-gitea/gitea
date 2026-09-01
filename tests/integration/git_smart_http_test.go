// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitSmartHTTP(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		testGitSmartHTTP(t, u)
		testGitSmartHTTPTokenScopes(t)
		testRenamedRepoRedirect(t)
		testGitArchiveRemote(t, u)
		t.Run("AnonymousAccess-Repo", func(t *testing.T) { testGitSmartHTTPPrivateRepoAnonymousAccess(t, false) })
		t.Run("AnonymousAccess-Wiki", func(t *testing.T) { testGitSmartHTTPPrivateRepoAnonymousAccess(t, true) })
	})
}

func testGitSmartHTTP(t *testing.T, u *url.URL) {
	kases := []struct {
		method, path string
		code         int
	}{
		{
			path: "user2/repo1/info/refs",
			code: http.StatusOK,
		},
		{
			method: "HEAD",
			path:   "user2/repo1/info/refs",
			code:   http.StatusOK,
		},
		{
			path: "user2/repo1/HEAD",
			code: http.StatusOK,
		},
		{
			path: "user2/repo1/objects/info/alternates",
			code: http.StatusNotFound,
		},
		{
			path: "user2/repo1/objects/info/http-alternates",
			code: http.StatusNotFound,
		},
		{
			path: "user2/repo1/../../custom/conf/app.ini",
			code: http.StatusNotFound,
		},
		{
			path: "user2/repo1/objects/info/../../../../custom/conf/app.ini",
			code: http.StatusNotFound,
		},
		{
			path: `user2/repo1/objects/info/..\..\..\..\custom\conf\app.ini`,
			code: http.StatusBadRequest,
		},
	}

	for _, kase := range kases {
		t.Run(kase.path, func(t *testing.T) {
			req, err := http.NewRequest(util.IfZero(kase.method, "GET"), u.String()+kase.path, nil)
			require.NoError(t, err)
			req.SetBasicAuth("user2", userPassword)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, kase.code, resp.StatusCode)
			_, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
		})
	}
}

func testGitSmartHTTPTokenScopes(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2, OwnerName: "user2", Name: "repo2"})
	require.True(t, repo.IsPrivate)

	session := loginUser(t, "user2")
	badToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadNotification)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	publicOnlyToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopePublicOnly, auth_model.AccessTokenScopeReadRepository)

	t.Run("upload-pack requires read repository scope", func(t *testing.T) {
		path := "/user2/repo2/info/refs?service=git-upload-pack"

		MakeRequest(t, NewRequest(t, "GET", path).AddBasicAuth(badToken, "x-oauth-basic"), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(badToken), http.StatusForbidden)

		resp := MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(readToken), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "refs/heads/master")
	})

	t.Run("receive-pack requires write repository scope", func(t *testing.T) {
		path := "/user2/repo2/info/refs?service=git-receive-pack"

		MakeRequest(t, NewRequest(t, "GET", path).AddBasicAuth(readToken, "x-oauth-basic"), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(readToken), http.StatusForbidden)

		resp := MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(writeToken), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "refs/heads/master")
	})

	t.Run("public-only scope rejects private repo", func(t *testing.T) {
		path := "/user2/repo2/info/refs?service=git-upload-pack"
		MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(publicOnlyToken), http.StatusForbidden)
	})
}

func testRenamedRepoRedirect(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()

	// git client requires to get a 301 redirect response before 401 unauthorized response
	req := NewRequest(t, "GET", "/user2/oldrepo1/info/refs")
	resp := MakeRequest(t, req, http.StatusMovedPermanently)
	redirect := resp.Header().Get("Location")
	assert.Equal(t, "/user2/repo1/info/refs", redirect)

	req = NewRequest(t, "GET", redirect)
	resp = MakeRequest(t, req, http.StatusUnauthorized)
	assert.Equal(t, "Unauthorized\n", resp.Body.String())

	req = NewRequest(t, "GET", redirect).AddBasicAuth("user2")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "65f1bf27bc3bf70f64657658635e66094edbcb4d\trefs/tags/v1.1")
}

func testGitArchiveRemote(t *testing.T, u *url.URL) {
	u = u.JoinPath("user27/repo49.git")
	t.Run("Fetch HEAD archive", doGitRemoteArchive(u.String(), "HEAD"))
	t.Run("Fetch HEAD archive subpath", doGitRemoteArchive(u.String(), "HEAD", "test"))
	t.Run("list compression options", doGitRemoteArchive(u.String(), "--list"))
}

// testGitSmartHTTPPrivateRepoAnonymousAccess tests that a private repo with
// anonymous code access enabled can be cloned without credentials.
func testGitSmartHTTPPrivateRepoAnonymousAccess(t *testing.T, isWiki bool) {
	// repo1 (ID=1) belongs to user2 and is public by default in fixtures
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerName: "user2", Name: "repo1"})
	unitType := util.Iif(isWiki, unit.TypeWiki, unit.TypeCode)
	repoLink := "/" + repo.FullName() + util.Iif(isWiki, ".wiki", "")
	gitPullPath := repoLink + "/info/refs?service=git-upload-pack"
	gitPushPath := repoLink + "/info/refs?service=git-receive-pack"

	// make the repo private
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), &repo_model.Repository{ID: repo.ID, IsPrivate: true}, "is_private"))

	// without anonymous access: anonymous pull must require auth
	MakeRequest(t, NewRequest(t, "GET", gitPullPath), http.StatusUnauthorized)

	// enable anonymous read access on the unit
	require.NoError(t, repo_model.UpdateRepoUnitPublicAccess(t.Context(), &repo_model.RepoUnit{RepoID: repo.ID, Type: unitType, AnonymousAccessMode: perm.AccessModeRead}))

	// with anonymous code access: anonymous pull must succeed without credentials
	MakeRequest(t, NewRequest(t, "GET", gitPullPath), http.StatusOK)

	// push (receive-pack) must still require auth even with anonymous code access
	MakeRequest(t, NewRequest(t, "GET", gitPushPath), http.StatusUnauthorized)

	// RequireSignInViewStrict must override anonymous access
	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()
	MakeRequest(t, NewRequest(t, "GET", gitPullPath), http.StatusUnauthorized)
}
