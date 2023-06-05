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
)

func TestAPIRepoTags(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	repoName := "repo1"

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/tags?token=%s", user.Name, repoName, token)
	resp := MakeRequest(t, req, http.StatusOK)

	var tags []*api.Tag
	DecodeJSON(t, resp, &tags)

	assert.Len(t, tags, 1)
	assert.Equal(t, "v1.1", tags[0].Name)
	assert.Equal(t, "Initial commit", tags[0].Message)
	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", tags[0].Commit.SHA)
	assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/git/commits/65f1bf27bc3bf70f64657658635e66094edbcb4d", tags[0].Commit.URL)
	assert.Equal(t, setting.AppURL+"user2/repo1/archive/v1.1.zip", tags[0].ZipballURL)
	assert.Equal(t, setting.AppURL+"user2/repo1/archive/v1.1.tar.gz", tags[0].TarballURL)

	newTag := createNewTagUsingAPI(t, session, token, user.Name, repoName, "gitea/22", "", "nice!\nand some text")
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &tags)
	assert.Len(t, tags, 2)
	for _, tag := range tags {
		if tag.Name != "v1.1" {
			assert.EqualValues(t, newTag.Name, tag.Name)
			assert.EqualValues(t, newTag.Message, tag.Message)
			assert.EqualValues(t, "nice!\nand some text", tag.Message)
			assert.EqualValues(t, newTag.Commit.SHA, tag.Commit.SHA)
		}
	}

	// get created tag
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/tags/%s?token=%s", user.Name, repoName, newTag.Name, token)
	resp = MakeRequest(t, req, http.StatusOK)
	var tag *api.Tag
	DecodeJSON(t, resp, &tag)
	assert.EqualValues(t, newTag, tag)

	// delete tag
	delReq := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/tags/%s?token=%s", user.Name, repoName, newTag.Name, token)
	MakeRequest(t, delReq, http.StatusNoContent)

	// check if it's gone
	MakeRequest(t, req, http.StatusNotFound)
}

func createNewTagUsingAPI(t *testing.T, session *TestSession, token, ownerName, repoName, name, target, msg string) *api.Tag {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/tags?token=%s", ownerName, repoName, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateTagOption{
		TagName: name,
		Message: msg,
		Target:  target,
	})
	resp := MakeRequest(t, req, http.StatusCreated)

	var respObj api.Tag
	DecodeJSON(t, resp, &respObj)
	return &respObj
}
