// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPICreateRelease(t *testing.T) {
	prepareTestEnv(t)

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session)
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	assert.NoError(t, err)

	err = gitRepo.CreateTag("v0.0.1", "master")
	assert.NoError(t, err)

	commitID, err := gitRepo.GetTagCommitID("v0.0.1")
	assert.NoError(t, err)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/releases?token=%s",
		owner.Name, repo.Name, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateReleaseOption{
		TagName:      "v0.0.1",
		Title:        "v0.0.1",
		Note:         "test",
		IsDraft:      false,
		IsPrerelease: false,
		Target:       commitID,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newRelease api.Release
	DecodeJSON(t, resp, &newRelease)
	models.AssertExistsAndLoadBean(t, &models.Release{
		ID:      newRelease.ID,
		TagName: newRelease.TagName,
		Title:   newRelease.Title,
		Note:    newRelease.Note,
	})

	urlStr = fmt.Sprintf("/api/v1/repos/%s/%s/releases/%d?token=%s",
		owner.Name, repo.Name, newRelease.ID, token)
	req = NewRequest(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)

	var release api.Release
	DecodeJSON(t, resp, &release)

	assert.Equal(t, newRelease.TagName, release.TagName)
	assert.Equal(t, newRelease.Title, release.Title)
	assert.Equal(t, newRelease.Note, release.Note)

	req = NewRequestWithJSON(t, "PATCH", urlStr, &api.EditReleaseOption{
		TagName:      release.TagName,
		Title:        release.Title,
		Note:         "updated",
		IsDraft:      &release.IsDraft,
		IsPrerelease: &release.IsPrerelease,
		Target:       release.Target,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &newRelease)
	models.AssertExistsAndLoadBean(t, &models.Release{
		ID:      newRelease.ID,
		TagName: newRelease.TagName,
		Title:   newRelease.Title,
		Note:    newRelease.Note,
	})
}
