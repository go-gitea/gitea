// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPISearchCodeNotLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// test with no keyword
	req := NewRequest(t, "GET", "/api/v1/search/code")
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequest(t, "GET", "/api/v1/search/code?q=Description")
	resp := MakeRequest(t, req, http.StatusOK)

	var apiCodeSearchResults api.CodeSearchResults
	DecodeJSON(t, resp, &apiCodeSearchResults)
	assert.Equal(t, int64(1), apiCodeSearchResults.TotalCount)
	assert.Len(t, apiCodeSearchResults.Items, 1)
	assert.Equal(t, "README.md", apiCodeSearchResults.Items[0].Name)
	assert.Equal(t, "README.md", apiCodeSearchResults.Items[0].Path)
	assert.Equal(t, "Markdown", apiCodeSearchResults.Items[0].Language)
	assert.Len(t, apiCodeSearchResults.Items[0].Lines, 2)
	assert.Equal(t, "\n", apiCodeSearchResults.Items[0].Lines[0].RawContent)
	assert.Equal(t, "Description for repo1", apiCodeSearchResults.Items[0].Lines[1].RawContent)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo1, err := gitrepo.OpenRepository(t.Context(), repo1)
	assert.NoError(t, err)
	defer gitRepo1.Close()

	commitID, err := gitRepo1.GetBranchCommitID(repo1.DefaultBranch)
	assert.NoError(t, err)

	assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/contents/README.md?ref="+commitID, apiCodeSearchResults.Items[0].URL)
	assert.Equal(t, setting.AppURL+"user2/repo1/blob/"+commitID+"/README.md", apiCodeSearchResults.Items[0].HTMLURL)

	assert.Equal(t, int64(1), apiCodeSearchResults.Items[0].Repository.ID)

	assert.Len(t, apiCodeSearchResults.Languages, 1)
	assert.Equal(t, "Markdown", apiCodeSearchResults.Languages[0].Language)
	assert.Equal(t, 1, apiCodeSearchResults.Languages[0].Count)
}
