// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func compareCommitFiles(t *testing.T, expect []string, files []*api.CommitAffectedFiles) {
	var actual []string
	for i := range files {
		actual = append(actual, files[i].Filename)
	}
	assert.ElementsMatch(t, expect, actual)
}

func TestAPIReposGitCommits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// check invalid requests
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/commits/12345", user.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/commits/..", user.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/commits/branch-not-exist", user.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	for _, ref := range [...]string{
		"master", // Branch
		"v1.1",   // Tag
		"65f1",   // short sha
		"65f1bf27bc3bf70f64657658635e66094edbcb4d", // full sha
	} {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/commits/%s", user.Name, ref).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	}
}

func TestAPIReposGitCommitList(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test getting commits (Page 1)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo20/commits?not=master&sha=remove-files-a", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Len(t, apiData, 2)
	assert.EqualValues(t, "cfe3b3c1fd36fba04f9183287b106497e1afe986", apiData[0].CommitMeta.SHA)
	compareCommitFiles(t, []string{"link_hi", "test.csv"}, apiData[0].Files)
	assert.EqualValues(t, "c8e31bc7688741a5287fcde4fbb8fc129ca07027", apiData[1].CommitMeta.SHA)
	compareCommitFiles(t, []string{"test.csv"}, apiData[1].Files)

	assert.EqualValues(t, "2", resp.Header().Get("X-Total"))
}

func TestAPIReposGitCommitListNotMaster(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test getting commits (Page 1)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/commits", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Len(t, apiData, 3)
	assert.EqualValues(t, "69554a64c1e6030f051e5c3f94bfbd773cd6a324", apiData[0].CommitMeta.SHA)
	compareCommitFiles(t, []string{"readme.md"}, apiData[0].Files)
	assert.EqualValues(t, "27566bd5738fc8b4e3fef3c5e72cce608537bd95", apiData[1].CommitMeta.SHA)
	compareCommitFiles(t, []string{"readme.md"}, apiData[1].Files)
	assert.EqualValues(t, "5099b81332712fe655e34e8dd63574f503f61811", apiData[2].CommitMeta.SHA)
	compareCommitFiles(t, []string{"readme.md"}, apiData[2].Files)

	assert.EqualValues(t, "3", resp.Header().Get("X-Total"))
}

func TestAPIReposGitCommitListPage2Empty(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test getting commits (Page=2)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/commits?page=2", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Empty(t, apiData)
}

func TestAPIReposGitCommitListDifferentBranch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test getting commits (Page=1, Branch=good-sign)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/commits?sha=good-sign", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Len(t, apiData, 1)
	assert.Equal(t, "f27c2b2b03dcab38beaf89b0ab4ff61f6de63441", apiData[0].CommitMeta.SHA)
	compareCommitFiles(t, []string{"readme.md"}, apiData[0].Files)
}

func TestAPIReposGitCommitListWithoutSelectFields(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test getting commits without files, verification, and stats
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/commits?sha=good-sign&stat=false&files=false&verification=false", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Len(t, apiData, 1)
	assert.Equal(t, "f27c2b2b03dcab38beaf89b0ab4ff61f6de63441", apiData[0].CommitMeta.SHA)
	assert.Equal(t, (*api.CommitStats)(nil), apiData[0].Stats)
	assert.Equal(t, (*api.PayloadCommitVerification)(nil), apiData[0].RepoCommit.Verification)
	assert.Equal(t, ([]*api.CommitAffectedFiles)(nil), apiData[0].Files)
}

func TestDownloadCommitDiffOrPatch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// Test getting diff
	reqDiff := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/git/commits/f27c2b2b03dcab38beaf89b0ab4ff61f6de63441.diff", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, reqDiff, http.StatusOK)
	assert.EqualValues(t,
		"commit f27c2b2b03dcab38beaf89b0ab4ff61f6de63441\nAuthor: User2 <user2@example.com>\nDate:   Sun Aug 6 19:55:01 2017 +0200\n\n    good signed commit\n\ndiff --git a/readme.md b/readme.md\nnew file mode 100644\nindex 0000000..458121c\n--- /dev/null\n+++ b/readme.md\n@@ -0,0 +1 @@\n+good sign\n",
		resp.Body.String())

	// Test getting patch
	reqPatch := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/git/commits/f27c2b2b03dcab38beaf89b0ab4ff61f6de63441.patch", user.Name).
		AddTokenAuth(token)
	resp = MakeRequest(t, reqPatch, http.StatusOK)
	assert.EqualValues(t,
		"From f27c2b2b03dcab38beaf89b0ab4ff61f6de63441 Mon Sep 17 00:00:00 2001\nFrom: User2 <user2@example.com>\nDate: Sun, 6 Aug 2017 19:55:01 +0200\nSubject: [PATCH] good signed commit\n\n---\n readme.md | 1 +\n 1 file changed, 1 insertion(+)\n create mode 100644 readme.md\n\ndiff --git a/readme.md b/readme.md\nnew file mode 100644\nindex 0000000..458121c\n--- /dev/null\n+++ b/readme.md\n@@ -0,0 +1 @@\n+good sign\n",
		resp.Body.String())
}

func TestGetFileHistory(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo16/commits?path=readme.md&sha=good-sign", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Len(t, apiData, 1)
	assert.Equal(t, "f27c2b2b03dcab38beaf89b0ab4ff61f6de63441", apiData[0].CommitMeta.SHA)
	compareCommitFiles(t, []string{"readme.md"}, apiData[0].Files)

	assert.EqualValues(t, "1", resp.Header().Get("X-Total"))
}

func TestGetFileHistoryNotOnMaster(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo20/commits?path=test.csv&sha=add-csv&not=master", user.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiData []api.Commit
	DecodeJSON(t, resp, &apiData)

	assert.Len(t, apiData, 1)
	assert.Equal(t, "c8e31bc7688741a5287fcde4fbb8fc129ca07027", apiData[0].CommitMeta.SHA)
	compareCommitFiles(t, []string{"test.csv"}, apiData[0].Files)

	assert.EqualValues(t, "1", resp.Header().Get("X-Total"))
}
