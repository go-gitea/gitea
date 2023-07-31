// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIReposGitCompare(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		// Login as User2.
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		// check invalid request
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/compare/master...unknow_branch?token=%s", user.Name, token)
		MakeRequest(t, req, http.StatusNotFound)

		// check valid requests
		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/compare/master...branch2?token=%s", user.Name, token)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiData api.GitCompareResponse
		DecodeJSON(t, resp, &apiData)
		assert.Equal(t, 2, len(apiData.Commits))
		assert.Equal(t, 1, len(apiData.Files))
		assert.Equal(t, "README.md", apiData.Files[0].FileName)
		assert.Equal(t, "modified", apiData.Files[0].Status)
		diffPatch := `@@ -1,3 +1,6 @@
 # repo1
 
-Description for repo1
+Description for repo1
+
+And change for branch2
+and a second one
 
`
		assert.Equal(t, diffPatch, apiData.Files[0].Patch)
		assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", apiData.MergeBaseCommit.SHA)
		apiData.Files = nil

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/compare/master...branch2?skip-patch=true&token=%s", user.Name, token)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &apiData)
		assert.Equal(t, "README.md", apiData.Files[0].FileName)
		assert.Equal(t, "modified", apiData.Files[0].Status)
		assert.Equal(t, "", apiData.Files[0].Patch)
	})
}
