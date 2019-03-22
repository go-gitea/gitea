// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"code.gitea.io/gitea/modules/repofiles"
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
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
		DeleteFileOptions: *getDeleteFileOptions(),
		Content:           contentEncoded,
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

func getExpectedFileContentResponseForFileContents(branch string) *api.FileContentResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	return &api.FileContentResponse{
		Name:        filepath.Base(treePath),
		Path:        treePath,
		SHA:         sha,
		Size:        30,
		URL:         "http://localhost:3003/api/v1/repos/user2/repo1/contents/" + treePath,
		HTMLURL:     "http://localhost:3003/user2/repo1/blob/" + branch + "/" + treePath,
		GitURL:      "http://localhost:3003/api/v1/repos/user2/repo1/git/blobs/" + sha,
		DownloadURL: "http://localhost:3003/user2/repo1/raw/branch/" + branch + "/" + treePath,
		Type:        "blob",
		Links: &api.FileLinksResponse{
			Self:    "http://localhost:3003/api/v1/repos/user2/repo1/contents/" + treePath,
			GitURL:  "http://localhost:3003/api/v1/repos/user2/repo1/git/blobs/" + sha,
			HTMLURL: "http://localhost:3003/user2/repo1/blob/" + branch + "/" + treePath,
		},
	}
}

func getDeleteFileOptions() *api.DeleteFileOptions {
	return &api.DeleteFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "Updates new/file.txt",
			Author: api.Identity{
				Name:  "John Doe",
				Email: "johndoe@example.com",
			},
			Committer: api.Identity{
				Name:  "Jane Doe",
				Email: "janedoe@example.com",
			},
		},
		SHA: "103ff9234cefeee5ec5361d22b49fbb04d385885",
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

func createFileInBranch(user *models.User, repo *models.Repository, treePath, branchName string) (*api.FileResponse, error) {
	opts := &repofiles.UpdateRepoFileOptions{
		OldBranch: branchName,
		TreePath:  treePath,
		Content:   "This is a NEW file",
		IsNewFile: true,
		Author:    nil,
		Committer: nil,
	}
	return repofiles.CreateOrUpdateRepoFile(repo, user, opts)
}

func createFile(user *models.User, repo *models.Repository, treePath string) (*api.FileResponse, error) {
	return createFileInBranch(user, repo, treePath, repo.DefaultBranch)
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
		createFile(user2, repo1, treePath)
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
	createFile(user2, repo1, treePath)
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
	createFile(user2, repo16, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Tests a repo with no token given so will fail
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFile(user2, repo16, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using access token for a private repo that the user of the token owns
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFile(user2, repo16, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token2)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFile(user3, repo3, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" with no user token
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFile(user3, repo3, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user3.Name, repo3.Name, treePath)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using repo "user2/repo1" where user4 is a NOT collaborator
	fileID++
	treePath = fmt.Sprintf("update/file%d.txt", fileID)
	createFile(user2, repo1, treePath)
	updateFileOptions = getUpdateFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token4)
	req = NewRequestWithJSON(t, "PUT", url, &updateFileOptions)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIDeleteFile(t *testing.T) {
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

	// Test deleting a file in repo1 which user2 owns, try both with branch and empty branch
	for _, branch := range [...]string{
		"master", // Branch
		"",       // Empty branch
	} {
		fileID++
		treePath := fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		deleteFileOptions := getDeleteFileOptions()
		deleteFileOptions.BranchName = branch
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req := NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
		resp := session.MakeRequest(t, req, http.StatusOK)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		assert.NotNil(t, fileResponse)
		assert.Nil(t, fileResponse.Content)
	}

	// Test deleting a file with the wrong SHA
	fileID++
	treePath := fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user2, repo1, treePath)
	deleteFileOptions := getDeleteFileOptions()
	correctSHA := deleteFileOptions.SHA
	deleteFileOptions.SHA = "badsha"
	url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
	req := NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	resp := session.MakeRequest(t, req, http.StatusInternalServerError)
	expectedAPIError := context.APIError{
		Message: "sha does not match [given: " + deleteFileOptions.SHA + ", expected: " + correctSHA + "]",
		URL:     base.DocURL,
	}
	var apiError context.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, expectedAPIError, apiError)

	// Test creating a file in repo1 by user4 who does not have write access
	fileID++
	treePath = fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user2, repo16, treePath)
	deleteFileOptions = getDeleteFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Tests a repo with no token given so will fail
	fileID++
	treePath = fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user2, repo16, treePath)
	deleteFileOptions = getDeleteFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath)
	req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using access token for a private repo that the user of the token owns
	fileID++
	treePath = fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user2, repo16, treePath)
	deleteFileOptions = getDeleteFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token2)
	req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" where user2 is a collaborator
	fileID++
	treePath = fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user3, repo3, treePath)
	deleteFileOptions = getDeleteFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	session.MakeRequest(t, req, http.StatusOK)

	// Test using org repo "user3/repo3" with no user token
	fileID++
	treePath = fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user3, repo3, treePath)
	deleteFileOptions = getDeleteFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user3.Name, repo3.Name, treePath)
	req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test using repo "user2/repo1" where user4 is a NOT collaborator
	fileID++
	treePath = fmt.Sprintf("delete/file%d.txt", fileID)
	createFile(user2, repo1, treePath)
	deleteFileOptions = getDeleteFileOptions()
	url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token4)
	req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIGetFileContents(t *testing.T) {
	prepareTestEnv(t)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	treePath := "README.md"

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)

	// Make a second master branch in repo1
	repo1.CreateNewBranch(user2, repo1.DefaultBranch, "master2")

	// ref is default branch
	branch := repo1.DefaultBranch
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, branch)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var fileContentResponse api.FileContentResponse
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse := getExpectedFileContentResponseForFileContents(branch)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// No ref
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents(repo1.DefaultBranch)
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// ref is master2
	branch = "master2"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, branch)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &fileContentResponse)
	assert.NotNil(t, fileContentResponse)
	expectedFileContentResponse = getExpectedFileContentResponseForFileContents("master2")
	assert.EqualValues(t, *expectedFileContentResponse, fileContentResponse)

	// Test file contents a file with the wrong branch
	branch = "badbranch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, branch)
	resp = session.MakeRequest(t, req, http.StatusInternalServerError)
	expectedAPIError := context.APIError{
		Message: "object does not exist [id: " + branch + ", rel_path: ]",
		URL:     base.DocURL,
	}
	var apiError context.APIError
	DecodeJSON(t, resp, &apiError)
	assert.Equal(t, expectedAPIError, apiError)

	// Test accessing private branch with user token that does not have access - should fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test access private branch of owner of token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/readme.md?token=%s", user2.Name, repo16.Name, token2)
	session.MakeRequest(t, req, http.StatusOK)

	// Test access of org user3 private repo file by owner user2
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	session.MakeRequest(t, req, http.StatusOK)
}
