// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoTags(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	repoName := "repo1"

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/tags", user.Name, repoName).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	tags := DecodeJSON(t, resp, []*api.Tag{})

	assert.Len(t, tags, 1)
	assert.Equal(t, "v1.1", tags[0].Name)
	assert.Equal(t, "Initial commit\n", tags[0].Message)
	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", tags[0].Commit.SHA)
	assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d", tags[0].Commit.URL)
	assert.Equal(t, setting.AppURL+"user2/repo1/archive/v1.1.zip", tags[0].ZipballURL)
	assert.Equal(t, setting.AppURL+"user2/repo1/archive/v1.1.tar.gz", tags[0].TarballURL)

	newTag := createNewTagUsingAPI(t, token, user.Name, repoName, "gitea/22", "", "nice!\nand some text")
	assert.Equal(t, "nice!\nand some text\n", newTag.Message) // git message standard: there will always be a newline at the end of the message

	resp = MakeRequest(t, req, http.StatusOK)
	tags = DecodeJSON(t, resp, []*api.Tag{})
	require.Len(t, tags, 2)
	respTag := tags[0]
	assert.Equal(t, newTag.Name, respTag.Name)
	assert.Equal(t, newTag.Message, respTag.Message)
	assert.Equal(t, newTag.Commit.SHA, respTag.Commit.SHA)

	// get created tag
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/tags/%s", user.Name, repoName, newTag.Name).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	tag := DecodeJSON(t, resp, &api.Tag{})
	assert.Equal(t, newTag, tag)

	// delete tag
	delReq := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/tags/%s", user.Name, repoName, newTag.Name).
		AddTokenAuth(token)
	MakeRequest(t, delReq, http.StatusNoContent)

	// check if it's gone
	MakeRequest(t, req, http.StatusNotFound)
}

func createNewTagUsingAPI(t *testing.T, token, ownerName, repoName, name, target, msg string) *api.Tag {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/tags", ownerName, repoName)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateTagOption{
		TagName: name,
		Message: msg,
		Target:  target,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	respObj := DecodeJSON(t, resp, &api.Tag{})
	return respObj
}
