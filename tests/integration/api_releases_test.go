// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListReleases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeReadRepository)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/releases", user2.Name, repo.Name))
	resp := MakeRequest(t, NewRequest(t, "GET", link.String()).AddTokenAuth(token), http.StatusOK)
	var apiReleases []*api.Release
	DecodeJSON(t, resp, &apiReleases)
	if assert.Len(t, apiReleases, 3) {
		for _, release := range apiReleases {
			switch release.ID {
			case 1:
				assert.False(t, release.IsDraft)
				assert.False(t, release.IsPrerelease)
				assert.True(t, strings.HasSuffix(release.UploadURL, "/api/v1/repos/user2/repo1/releases/1/assets"), release.UploadURL)
			case 4:
				assert.True(t, release.IsDraft)
				assert.False(t, release.IsPrerelease)
				assert.True(t, strings.HasSuffix(release.UploadURL, "/api/v1/repos/user2/repo1/releases/4/assets"), release.UploadURL)
			case 5:
				assert.False(t, release.IsDraft)
				assert.True(t, release.IsPrerelease)
				assert.True(t, strings.HasSuffix(release.UploadURL, "/api/v1/repos/user2/repo1/releases/5/assets"), release.UploadURL)
			default:
				assert.NoError(t, fmt.Errorf("unexpected release: %v", release))
			}
		}
	}

	// test filter
	testFilterByLen := func(auth bool, query url.Values, expectedLength int, msgAndArgs ...string) {
		link.RawQuery = query.Encode()
		req := NewRequest(t, "GET", link.String())
		if auth {
			req.AddTokenAuth(token)
		}
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &apiReleases)
		assert.Len(t, apiReleases, expectedLength, msgAndArgs)
	}

	testFilterByLen(false, url.Values{"draft": {"true"}}, 0, "anon should not see drafts")
	testFilterByLen(true, url.Values{"draft": {"true"}}, 1, "repo owner should see drafts")
	testFilterByLen(true, url.Values{"draft": {"false"}}, 2, "exclude drafts")
	testFilterByLen(true, url.Values{"draft": {"false"}, "pre-release": {"false"}}, 1, "exclude drafts and pre-releases")
	testFilterByLen(true, url.Values{"pre-release": {"true"}}, 1, "only get pre-release")
	testFilterByLen(true, url.Values{"draft": {"true"}, "pre-release": {"true"}}, 0, "there is no pre-release draft")
}

func createNewReleaseUsingAPI(t *testing.T, token string, owner *user_model.User, repo *repo_model.Repository, name, target, title, desc string) *api.Release {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/releases", owner.Name, repo.Name)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateReleaseOption{
		TagName:      name,
		Title:        title,
		Note:         desc,
		IsDraft:      false,
		IsPrerelease: false,
		Target:       target,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	var newRelease api.Release
	DecodeJSON(t, resp, &newRelease)
	rel := &repo_model.Release{
		ID:      newRelease.ID,
		TagName: newRelease.TagName,
		Title:   newRelease.Title,
	}
	unittest.AssertExistsAndLoadBean(t, rel)
	assert.EqualValues(t, newRelease.Note, rel.Note)

	return &newRelease
}

func TestAPICreateAndUpdateRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	err = gitRepo.CreateTag("v0.0.1", "master")
	assert.NoError(t, err)

	target, err := gitRepo.GetTagCommitID("v0.0.1")
	assert.NoError(t, err)

	newRelease := createNewReleaseUsingAPI(t, token, owner, repo, "v0.0.1", target, "v0.0.1", "test")

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/releases/%d", owner.Name, repo.Name, newRelease.ID)
	req := NewRequest(t, "GET", urlStr).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

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
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &newRelease)
	rel := &repo_model.Release{
		ID:      newRelease.ID,
		TagName: newRelease.TagName,
		Title:   newRelease.Title,
	}
	unittest.AssertExistsAndLoadBean(t, rel)
	assert.EqualValues(t, rel.Note, newRelease.Note)
}

func TestAPICreateProtectedTagRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	writer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, writer.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit("master")
	assert.NoError(t, err)

	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/releases", repo.OwnerName, repo.Name), &api.CreateReleaseOption{
		TagName:      "v0.0.1",
		Title:        "v0.0.1",
		IsDraft:      false,
		IsPrerelease: false,
		Target:       commit.ID.String(),
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPICreateReleaseToDefaultBranch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	createNewReleaseUsingAPI(t, token, owner, repo, "v0.0.1", "", "v0.0.1", "test")
}

func TestAPICreateReleaseToDefaultBranchOnExistingTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	err = gitRepo.CreateTag("v0.0.1", "master")
	assert.NoError(t, err)

	createNewReleaseUsingAPI(t, token, owner, repo, "v0.0.1", "", "v0.0.1", "test")
}

func TestAPICreateReleaseGivenInvalidTarget(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/releases", owner.Name, repo.Name)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateReleaseOption{
		TagName: "i-point-to-an-invalid-target",
		Title:   "Invalid Target",
		Target:  "invalid-target",
	}).AddTokenAuth(token)

	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIGetLatestRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/releases/latest", owner.Name, repo.Name))
	resp := MakeRequest(t, req, http.StatusOK)

	var release *api.Release
	DecodeJSON(t, resp, &release)

	assert.Equal(t, "testing-release", release.Title)
}

func TestAPIGetReleaseByTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	tag := "v1.1"

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/releases/tags/%s", owner.Name, repo.Name, tag))
	resp := MakeRequest(t, req, http.StatusOK)

	var release *api.Release
	DecodeJSON(t, resp, &release)

	assert.Equal(t, "testing-release", release.Title)

	nonexistingtag := "nonexistingtag"

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/releases/tags/%s", owner.Name, repo.Name, nonexistingtag))
	resp = MakeRequest(t, req, http.StatusNotFound)

	var err *api.APIError
	DecodeJSON(t, resp, &err)
	assert.NotEmpty(t, err.Message)
}

func TestAPIDeleteReleaseByTagName(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	createNewReleaseUsingAPI(t, token, owner, repo, "release-tag", "", "Release Tag", "test")

	// delete release
	req := NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/releases/tags/release-tag", owner.Name, repo.Name).
		AddTokenAuth(token)
	_ = MakeRequest(t, req, http.StatusNoContent)

	// make sure release is deleted
	req = NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/releases/tags/release-tag", owner.Name, repo.Name).
		AddTokenAuth(token)
	_ = MakeRequest(t, req, http.StatusNotFound)

	// delete release tag too
	req = NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/tags/release-tag", owner.Name, repo.Name).
		AddTokenAuth(token)
	_ = MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIUploadAssetRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	r := createNewReleaseUsingAPI(t, token, owner, repo, "release-tag", "", "Release Tag", "test")

	filename := "image.png"
	buff := generateImg()

	assetURL := fmt.Sprintf("/api/v1/repos/%s/%s/releases/%d/assets", owner.Name, repo.Name, r.ID)

	t.Run("multipart/form-data", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		body := &bytes.Buffer{}

		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("attachment", filename)
		assert.NoError(t, err)
		_, err = io.Copy(part, bytes.NewReader(buff.Bytes()))
		assert.NoError(t, err)
		err = writer.Close()
		assert.NoError(t, err)

		req := NewRequestWithBody(t, http.MethodPost, assetURL, bytes.NewReader(body.Bytes())).
			AddTokenAuth(token).
			SetHeader("Content-Type", writer.FormDataContentType())
		resp := MakeRequest(t, req, http.StatusCreated)

		var attachment *api.Attachment
		DecodeJSON(t, resp, &attachment)

		assert.EqualValues(t, filename, attachment.Name)
		assert.EqualValues(t, 104, attachment.Size)

		req = NewRequestWithBody(t, http.MethodPost, assetURL+"?name=test-asset", bytes.NewReader(body.Bytes())).
			AddTokenAuth(token).
			SetHeader("Content-Type", writer.FormDataContentType())
		resp = MakeRequest(t, req, http.StatusCreated)

		var attachment2 *api.Attachment
		DecodeJSON(t, resp, &attachment2)

		assert.EqualValues(t, "test-asset", attachment2.Name)
		assert.EqualValues(t, 104, attachment2.Size)
	})

	t.Run("application/octet-stream", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, http.MethodPost, assetURL, bytes.NewReader(buff.Bytes())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)

		req = NewRequestWithBody(t, http.MethodPost, assetURL+"?name=stream.bin", bytes.NewReader(buff.Bytes())).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var attachment *api.Attachment
		DecodeJSON(t, resp, &attachment)

		assert.EqualValues(t, "stream.bin", attachment.Name)
		assert.EqualValues(t, 104, attachment.Size)
	})
}
