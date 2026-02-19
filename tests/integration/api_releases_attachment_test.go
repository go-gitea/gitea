// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIEditReleaseAttachmentWithUnallowedFile(t *testing.T) {
	// Limit the allowed release types (since by default there is no restriction)
	defer test.MockVariableValue(&setting.Repository.Release.AllowedTypes, ".exe")()
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 9})
	release := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: attachment.ReleaseID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	filename := "file.bad"
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/releases/%d/assets/%d", repoOwner.Name, repo.Name, release.ID, attachment.ID)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"name": filename,
	}).AddTokenAuth(token)

	session.MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIDraftReleaseAttachmentAccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 13})
	release := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: attachment.ReleaseID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	reader := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	listURL := fmt.Sprintf("/api/v1/repos/%s/%s/releases/%d/assets", repoOwner.Name, repo.Name, release.ID)
	getURL := fmt.Sprintf("/api/v1/repos/%s/%s/releases/%d/assets/%d", repoOwner.Name, repo.Name, release.ID, attachment.ID)

	MakeRequest(t, NewRequest(t, "GET", listURL), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", getURL), http.StatusNotFound)

	readerToken := getUserToken(t, reader.LowerName, auth_model.AccessTokenScopeReadRepository)
	MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(readerToken), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", getURL).AddTokenAuth(readerToken), http.StatusNotFound)

	ownerReadToken := getUserToken(t, repoOwner.LowerName, auth_model.AccessTokenScopeReadRepository)
	MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(ownerReadToken), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", getURL).AddTokenAuth(ownerReadToken), http.StatusNotFound)
	ownerToken := getUserToken(t, repoOwner.LowerName, auth_model.AccessTokenScopeWriteRepository)
	resp := MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(ownerToken), http.StatusOK)
	var attachments []*api.Attachment
	DecodeJSON(t, resp, &attachments)
	if assert.Len(t, attachments, 1) {
		assert.Equal(t, attachment.ID, attachments[0].ID)
	}

	MakeRequest(t, NewRequest(t, "GET", getURL).AddTokenAuth(ownerToken), http.StatusOK)
}
