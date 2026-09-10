// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoGet(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/blah/glah/plah?go-get=1")
	resp := MakeRequest(t, req, http.StatusOK)

	expected := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%[1]s:%[2]s/blah/glah git %[3]sblah/glah.git">
		<meta name="go-source" content="%[1]s:%[2]s/blah/glah _ %[3]sblah/glah/src/branch/master{/dir} %[3]sblah/glah/src/branch/master{/dir}/{file}#L{line}">
	</head>
	<body>
		go get --insecure %[1]s:%[2]s/blah/glah
	</body>
</html>`, setting.Domain, setting.HTTPPort, setting.AppURL)

	assert.Equal(t, expected, resp.Body.String())
}

func TestGoGetForSSH(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Repository.GoGetCloneURLProtocol, "ssh")()

	req := NewRequest(t, "GET", "/blah/glah/plah?go-get=1")
	resp := MakeRequest(t, req, http.StatusOK)

	expected := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%[1]s:%[2]s/blah/glah git ssh://git@%[4]s:%[5]d/blah/glah.git">
		<meta name="go-source" content="%[1]s:%[2]s/blah/glah _ %[3]sblah/glah/src/branch/master{/dir} %[3]sblah/glah/src/branch/master{/dir}/{file}#L{line}">
	</head>
	<body>
		go get --insecure %[1]s:%[2]s/blah/glah
	</body>
</html>`, setting.Domain, setting.HTTPPort, setting.AppURL, setting.SSH.Domain, setting.SSH.Port)

	assert.Equal(t, expected, resp.Body.String())
}

// TestGoGetPrivateRepoBranchNotLeaked ensures the go-get meta endpoint does not disclose a
// private repository's default branch name to unauthorized callers.
func TestGoGetPrivateRepoBranchNotLeaked(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2/repo2 is private; give it a non-default branch name
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.True(t, repo.IsPrivate)
	repo.DefaultBranch = "secretbranch"
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo, "default_branch"))

	// an unauthenticated caller must see the neutral instance default, not the real branch
	req := NewRequest(t, "GET", "/user2/repo2?go-get=1")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "/src/branch/master{/dir}")
	assert.NotContains(t, resp.Body.String(), "secretbranch")

	// the owner may still see the real default branch
	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user2/repo2?go-get=1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "secretbranch")
}

// TestGoGetPublicRepoUnderLimitedOwnerBranchNotLeaked ensures the go-get meta endpoint does not disclose
// the default branch of a public repo whose owner is not visible to the caller (here a limited org, which
// is hidden from anonymous callers but visible to authenticated ones).
func TestGoGetPublicRepoUnderLimitedOwnerBranchNotLeaked(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// repo id 38 is a public repo owned by a limited org; give it a non-default branch name
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 38})
	require.False(t, repo.IsPrivate)
	require.NoError(t, repo.LoadOwner(t.Context()))
	require.False(t, repo.Owner.Visibility.IsPublic())
	repo.DefaultBranch = "secretbranch"
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo, "default_branch"))

	url := fmt.Sprintf("/%s/%s?go-get=1", repo.OwnerName, repo.Name)

	// an anonymous caller cannot see the limited owner, so the neutral instance default is returned
	resp := MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "/src/branch/master{/dir}")
	assert.NotContains(t, resp.Body.String(), "secretbranch")

	// an authenticated caller can see a limited org's public repo, so the real branch is shown
	session := loginUser(t, "user2")
	resp = session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "secretbranch")
}

// TestGoGetPrivateRepoBranchNotLeakedToTokenWithoutRepoScope ensures a PAT that was not granted repository
// read scope cannot learn a private repo's default branch through go-get, even when the account behind the
// token could read the repository.
func TestGoGetPrivateRepoBranchNotLeakedToTokenWithoutRepoScope(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2/repo2 is private; give it a non-default branch name
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.True(t, repo.IsPrivate)
	repo.DefaultBranch = "secretbranch"
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo, "default_branch"))

	// a token scoped only to read:misc does not grant repository read: the branch must stay hidden.
	// Web routes authenticate a token via basic auth (username + token), which also records the scope.
	miscToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadMisc)
	req := NewRequest(t, "GET", "/user2/repo2?go-get=1")
	req.Request.SetBasicAuth("user2", miscToken)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "/src/branch/master{/dir}")
	assert.NotContains(t, resp.Body.String(), "secretbranch")

	// a token that includes repository read scope may see the real branch
	repoToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	req = NewRequest(t, "GET", "/user2/repo2?go-get=1")
	req.Request.SetBasicAuth("user2", repoToken)
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "secretbranch")
}
