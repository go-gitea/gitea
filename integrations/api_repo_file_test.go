// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/repofiles"
	api "code.gitea.io/sdk/gitea"
)

func getCreateFileOptions() api.CreateFileOptions {
	content := "This is new text"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
	return api.CreateFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "Creates new/file.txt",
			Author: api.Identity{
				Name:  "John Doe",
				Email: "johndoe@example.com",
			},
			Committer: api.Identity{
				Name:  "Jane Doe",
				Email: "janedoe@example.com",
			},
		},
		Content: contentEncoded,
	}
}

func getExpectedFileResponseForCreate(commitID, treePath string) *api.FileResponse {
	sha := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
	return &api.FileResponse{
		Content: &api.FileContentResponse{
			Name:        filepath.Base(treePath),
			Path:        treePath,
			SHA:         sha,
			Size:        16,
			URL:         "http://localhost:3003/api/v1/repos/user2/repo1/contents/" + treePath,
			HTMLURL:     "http://localhost:3003/user2/repo1/blob/master/" + treePath,
			GitURL:      "http://localhost:3003/api/v1/repos/user2/repo1/git/blobs/" + sha,
			DownloadURL: "http://localhost:3003/user2/repo1/raw/branch/master/" + treePath,
			Type:        "blob",
			Links: &api.FileLinksResponse{
				Self:    "http://localhost:3003/api/v1/repos/user2/repo1/contents/" + treePath,
				GitURL:  "http://localhost:3003/api/v1/repos/user2/repo1/git/blobs/" + sha,
				HTMLURL: "http://localhost:3003/user2/repo1/blob/master/" + treePath,
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: "http://localhost:3003/api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: "http://localhost:3003/user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Jane Doe",
					Email: "janedoe@example.com",
				},
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
			},
			Message: "Updates README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "unsigned",
			Signature: "",
			Payload:   "",
		},
	}
}

func getUpdateFileOptions() *api.UpdateFileOptions {
	content := "This is updated text"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
	return &api.UpdateFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "Creates new/file.txt",
			Author: api.Identity{
				Name:  "John Doe",
				Email: "johndoe@example.com",
			},
			Committer: api.Identity{
				Name:  "Jane Doe",
				Email: "janedoe@example.com",
			},
		},
		SHA:     "103ff9234cefeee5ec5361d22b49fbb04d385885",
		Content: contentEncoded,
	}
}

func getExpectedFileResponseForUpdate(commitID, treePath string) *api.FileResponse {
	sha := "08bd14b2e2852529157324de9c226b3364e76136"
	return &api.FileResponse{
		Content: &api.FileContentResponse{
			Name:        filepath.Base(treePath),
			Path:        treePath,
			SHA:         sha,
			Size:        20,
			URL:         "http://localhost:3003/api/v1/repos/user2/repo1/contents/" + treePath,
			HTMLURL:     "http://localhost:3003/user2/repo1/blob/master/" + treePath,
			GitURL:      "http://localhost:3003/api/v1/repos/user2/repo1/git/blobs/" + sha,
			DownloadURL: "http://localhost:3003/user2/repo1/raw/branch/master/" + treePath,
			Type:        "blob",
			Links: &api.FileLinksResponse{
				Self:    "http://localhost:3003/api/v1/repos/user2/repo1/contents/" + treePath,
				GitURL:  "http://localhost:3003/api/v1/repos/user2/repo1/git/blobs/" + sha,
				HTMLURL: "http://localhost:3003/user2/repo1/blob/master/" + treePath,
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: "http://localhost:3003/api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: "http://localhost:3003/user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Jane Doe",
					Email: "janedoe@example.com",
				},
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
			},
			Message: "Updates README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "unsigned",
			Signature: "",
			Payload:   "",
		},
	}
}

func TestAPICreateFile(t *testing.T) {
	prepareTestEnv(t)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	fileID := 0

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)

	// Test creating a file in repo1 which user2 owns, try both with branch and empty branch
	for _, branch := range [...]string{
		"master", // Branch
		"",       // Empty branch
	} {
		createFileOptions := getCreateFileOptions()
		createFileOptions.BranchName = branch
		fileID++
		treePath := fmt.Sprintf("new/file%d.txt", fileID)
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req := NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp := session.MakeRequest(t, req, http.StatusCreated)
		gitRepo, _ := git.OpenRepository(repo1.RepoPath())
		commitID, _ := gitRepo.GetBranchCommitID(createFileOptions.NewBranchName)
		expectedFileResponse := getExpectedFileResponseForCreate(commitID, treePath)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
		assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
		assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
	}

	// Test trying to create a file that already exists, should fail
	createFileOptions := getCreateFileOptions()
	treePath := "README.md"
	url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
	req := NewRequestWithJSON(t, "POST", url, &createFileOptions)
	resp := session.MakeRequest(t, req, http.StatusInternalServerError)
	expectedAPIError := context.APIError{
		Message: "repository file already exists [path: " + treePath + "]",
		URL:     base.DocURL,
	}
	var apiError context.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, expectedAPIError, apiError)

	// Test creating a file in repo1 by user4 who does not have write access
	createFileOptions = getCreateFileOptions()
	fileID++
	treePath = fmt.Sprintf("new/file%d.txt", fileID)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Tests a repo with no token given so will fail
	createFileOptions = getCreateFileOptions()
	fileID++
	treePath = fmt.Sprintf("new/file%d.txt", fileID)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath)
	req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using access token for a private repo that the user of the token owns
	createFileOptions = getCreateFileOptions()
	fileID++
	treePath = fmt.Sprintf("new/file%d.txt", fileID)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token2)
	req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
	session.MakeRequest(t, req, http.StatusCreated)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	createFileOptions = getCreateFileOptions()
	fileID++
	treePath = fmt.Sprintf("new/file%d.txt", fileID)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
	session.MakeRequest(t, req, http.StatusCreated)

	// Test using org repo "user3/repo3" with no user token
	createFileOptions = getCreateFileOptions()
	fileID++
	treePath = fmt.Sprintf("new/file%d.txt", fileID)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user3.Name, repo3.Name, treePath)
	req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using repo "user2/repo1" where user4 is a NOT collaborator
	createFileOptions = getCreateFileOptions()
	fileID++
	treePath = fmt.Sprintf("new/file%d.txt", fileID)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token4)
	req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func createFileForUpdate(user *models.User, repo *models.Repository, treePath string) (*api.FileResponse, error) {
	opts := &repofiles.UpdateRepoFileOptions{
		OldBranch: repo.DefaultBranch,
		NewBranch: repo.DefaultBranch,
		TreePath:  treePath,
		Content:   "This is a NEW file",
		IsNewFile: true,
		Author:    nil,
		Committer: nil,
	}
	return repofiles.CreateOrUpdateRepoFile(repo, user, opts)
}

func TestAPIUpdateFile(t *testing.T) {
	prepareTestEnv(t)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	fileID := 0

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)

	// Test updating a file in repo1 which user2 owns, try both with branch and empty branch
	for _, branch := range [...]string{
		"master", // Branch
		"",       // Empty branch
	} {
		fileID++
		treePath := fmt.Sprintf("update/file%d.txt", fileID)
		createFileForUpdate(user2, repo1, treePath)
		updateFileOptions := getUpdateFileOptions()
		updateFileOptions.BranchName = branch
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req := NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
		resp := session.MakeRequest(t, req, http.StatusOK)
		gitRepo, _ := git.OpenRepository(repo1.RepoPath())
		commitID, _ := gitRepo.GetBranchCommitID(updateFileOptions.NewBranchName)
		expectedFileResponse := getExpectedFileResponseForUpdate(commitID, treePath)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
		assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
		assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
	}

	// Test updating a file with the wrong SHA
	fileID++
	treePath := fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user2, repo1, treePath)
	updateFileOptions := getUpdateFileOptions()
	correctSHA := updateFileOptions.SHA
	updateFileOptions.SHA = "badsha"
	url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
	req := NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	resp := session.MakeRequest(t, req, http.StatusInternalServerError)
	expectedAPIError := context.APIError{
		Message: "sha does not match [given: " + updateFileOptions.SHA + ", expected: " + correctSHA + "]",
		URL:     base.DocURL,
	}
	var apiError context.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, expectedAPIError, apiError)

	// Test creating a file in repo1 by user4 who does not have write access
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user2, repo16, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Tests a repo with no token given so will fail
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user2, repo16, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using access token for a private repo that the user of the token owns
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user2, repo16, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token2)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user3, repo3, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" with no user token
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user3, repo3, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user3.Name, repo3.Name, treePath)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using repo "user2/repo1" where user4 is a NOT collaborator
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFileForUpdate(user2, repo1, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token4)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusForbidden)
}
