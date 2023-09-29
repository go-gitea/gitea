// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	stdCtx "context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func getChangeFilesOptions() *api.ChangeFilesOptions {
	newContent := "This is new text"
	updateContent := "This is updated text"
	newContentEncoded := base64.StdEncoding.EncodeToString([]byte(newContent))
	updateContentEncoded := base64.StdEncoding.EncodeToString([]byte(updateContent))
	return &api.ChangeFilesOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "My update of new/file.txt",
			Author: api.Identity{
				Name:  "Anne Doe",
				Email: "annedoe@example.com",
			},
			Committer: api.Identity{
				Name:  "John Doe",
				Email: "johndoe@example.com",
			},
		},
		Files: []*api.ChangeFileOperation{
			{
				Operation:     "create",
				ContentBase64: newContentEncoded,
			},
			{
				Operation:     "update",
				ContentBase64: updateContentEncoded,
				SHA:           "103ff9234cefeee5ec5361d22b49fbb04d385885",
			},
			{
				Operation: "delete",
				SHA:       "103ff9234cefeee5ec5361d22b49fbb04d385885",
			},
		},
	}
}

func TestAPIChangeFiles(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
		org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3, is an org
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
		repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo
		fileID := 0

		// Get user2's token
		session := loginUser(t, user2.Name)
		token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		// Get user4's token
		session = loginUser(t, user4.Name)
		token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// Test changing files in repo1 which user2 owns, try both with branch and empty branch
		for _, branch := range [...]string{
			"master", // Branch
			"",       // Empty branch
		} {
			fileID++
			createTreePath := fmt.Sprintf("new/file%d.txt", fileID)
			updateTreePath := fmt.Sprintf("update/file%d.txt", fileID)
			deleteTreePath := fmt.Sprintf("delete/file%d.txt", fileID)
			createFile(user2, repo1, updateTreePath)
			createFile(user2, repo1, deleteTreePath)
			changeFilesOptions := getChangeFilesOptions()
			changeFilesOptions.BranchName = branch
			changeFilesOptions.Files[0].Path = createTreePath
			changeFilesOptions.Files[1].Path = updateTreePath
			changeFilesOptions.Files[2].Path = deleteTreePath
			url := fmt.Sprintf("/api/v1/repos/%s/%s/contents?token=%s", user2.Name, repo1.Name, token2)
			req := NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
			resp := MakeRequest(t, req, http.StatusCreated)
			gitRepo, _ := git.OpenRepository(stdCtx.Background(), repo1.RepoPath())
			commitID, _ := gitRepo.GetBranchCommitID(changeFilesOptions.NewBranchName)
			createLasCommit, _ := gitRepo.GetCommitByPath(createTreePath)
			updateLastCommit, _ := gitRepo.GetCommitByPath(updateTreePath)
			expectedCreateFileResponse := getExpectedFileResponseForCreate(fmt.Sprintf("%v/%v", user2.Name, repo1.Name), commitID, createTreePath, createLasCommit.ID.String())
			expectedUpdateFileResponse := getExpectedFileResponseForUpdate(commitID, updateTreePath, updateLastCommit.ID.String())
			var filesResponse api.FilesResponse
			DecodeJSON(t, resp, &filesResponse)

			// check create file
			assert.EqualValues(t, expectedCreateFileResponse.Content, filesResponse.Files[0])

			// check update file
			assert.EqualValues(t, expectedUpdateFileResponse.Content, filesResponse.Files[1])

			// test commit info
			assert.EqualValues(t, expectedCreateFileResponse.Commit.SHA, filesResponse.Commit.SHA)
			assert.EqualValues(t, expectedCreateFileResponse.Commit.HTMLURL, filesResponse.Commit.HTMLURL)
			assert.EqualValues(t, expectedCreateFileResponse.Commit.Author.Email, filesResponse.Commit.Author.Email)
			assert.EqualValues(t, expectedCreateFileResponse.Commit.Author.Name, filesResponse.Commit.Author.Name)
			assert.EqualValues(t, expectedCreateFileResponse.Commit.Committer.Email, filesResponse.Commit.Committer.Email)
			assert.EqualValues(t, expectedCreateFileResponse.Commit.Committer.Name, filesResponse.Commit.Committer.Name)

			// test delete file
			assert.Nil(t, filesResponse.Files[2])

			gitRepo.Close()
		}

		// Test changing files in a new branch
		changeFilesOptions := getChangeFilesOptions()
		changeFilesOptions.BranchName = repo1.DefaultBranch
		changeFilesOptions.NewBranchName = "new_branch"
		fileID++
		createTreePath := fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath := fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath := fmt.Sprintf("delete/file%d.txt", fileID)
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		createFile(user2, repo1, updateTreePath)
		createFile(user2, repo1, deleteTreePath)
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents?token=%s", user2.Name, repo1.Name, token2)
		req := NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		resp := MakeRequest(t, req, http.StatusCreated)
		var filesResponse api.FilesResponse
		DecodeJSON(t, resp, &filesResponse)
		expectedCreateSHA := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
		expectedCreateHTMLURL := fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/new_branch/new/file%d.txt", fileID)
		expectedCreateDownloadURL := fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/new_branch/new/file%d.txt", fileID)
		expectedUpdateSHA := "08bd14b2e2852529157324de9c226b3364e76136"
		expectedUpdateHTMLURL := fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/new_branch/update/file%d.txt", fileID)
		expectedUpdateDownloadURL := fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/new_branch/update/file%d.txt", fileID)
		assert.EqualValues(t, expectedCreateSHA, filesResponse.Files[0].SHA)
		assert.EqualValues(t, expectedCreateHTMLURL, *filesResponse.Files[0].HTMLURL)
		assert.EqualValues(t, expectedCreateDownloadURL, *filesResponse.Files[0].DownloadURL)
		assert.EqualValues(t, expectedUpdateSHA, filesResponse.Files[1].SHA)
		assert.EqualValues(t, expectedUpdateHTMLURL, *filesResponse.Files[1].HTMLURL)
		assert.EqualValues(t, expectedUpdateDownloadURL, *filesResponse.Files[1].DownloadURL)
		assert.Nil(t, filesResponse.Files[2])

		assert.EqualValues(t, changeFilesOptions.Message+"\n", filesResponse.Commit.Message)

		// Test updating a file and renaming it
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.BranchName = repo1.DefaultBranch
		fileID++
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, updateTreePath)
		changeFilesOptions.Files = []*api.ChangeFileOperation{changeFilesOptions.Files[1]}
		changeFilesOptions.Files[0].FromPath = updateTreePath
		changeFilesOptions.Files[0].Path = "rename/" + updateTreePath
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		resp = MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &filesResponse)
		expectedUpdateSHA = "08bd14b2e2852529157324de9c226b3364e76136"
		expectedUpdateHTMLURL = fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/master/rename/update/file%d.txt", fileID)
		expectedUpdateDownloadURL = fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/master/rename/update/file%d.txt", fileID)
		assert.EqualValues(t, expectedUpdateSHA, filesResponse.Files[0].SHA)
		assert.EqualValues(t, expectedUpdateHTMLURL, *filesResponse.Files[0].HTMLURL)
		assert.EqualValues(t, expectedUpdateDownloadURL, *filesResponse.Files[0].DownloadURL)

		// Test updating a file without a message
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Message = ""
		changeFilesOptions.BranchName = repo1.DefaultBranch
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		createFile(user2, repo1, updateTreePath)
		createFile(user2, repo1, deleteTreePath)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		resp = MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &filesResponse)
		expectedMessage := fmt.Sprintf("Add %v\nUpdate %v\nDelete %v\n", createTreePath, updateTreePath, deleteTreePath)
		assert.EqualValues(t, expectedMessage, filesResponse.Commit.Message)

		// Test updating a file with the wrong SHA
		fileID++
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, updateTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files = []*api.ChangeFileOperation{changeFilesOptions.Files[1]}
		changeFilesOptions.Files[0].Path = updateTreePath
		correctSHA := changeFilesOptions.Files[0].SHA
		changeFilesOptions.Files[0].SHA = "badsha"
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		resp = MakeRequest(t, req, http.StatusUnprocessableEntity)
		expectedAPIError := context.APIError{
			Message: "sha does not match [given: " + changeFilesOptions.Files[0].SHA + ", expected: " + correctSHA + "]",
			URL:     setting.API.SwaggerURL,
		}
		var apiError context.APIError
		DecodeJSON(t, resp, &apiError)
		assert.Equal(t, expectedAPIError, apiError)

		// Test creating a file in repo1 by user4 who does not have write access
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo16, updateTreePath)
		createFile(user2, repo16, deleteTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents?token=%s", user2.Name, repo16.Name, token4)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo16, updateTreePath)
		createFile(user2, repo16, deleteTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents", user2.Name, repo16.Name)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo16, updateTreePath)
		createFile(user2, repo16, deleteTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents?token=%s", user2.Name, repo16.Name, token2)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "org3/repo3" where user2 is a collaborator
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(org3, repo3, updateTreePath)
		createFile(org3, repo3, deleteTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents?token=%s", org3.Name, repo3.Name, token2)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "org3/repo3" with no user token
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(org3, repo3, updateTreePath)
		createFile(org3, repo3, deleteTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents", org3.Name, repo3.Name)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		fileID++
		createTreePath = fmt.Sprintf("new/file%d.txt", fileID)
		updateTreePath = fmt.Sprintf("update/file%d.txt", fileID)
		deleteTreePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo1, updateTreePath)
		createFile(user2, repo1, deleteTreePath)
		changeFilesOptions = getChangeFilesOptions()
		changeFilesOptions.Files[0].Path = createTreePath
		changeFilesOptions.Files[1].Path = updateTreePath
		changeFilesOptions.Files[2].Path = deleteTreePath
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents?token=%s", user2.Name, repo1.Name, token4)
		req = NewRequestWithJSON(t, "POST", url, &changeFilesOptions)
		MakeRequest(t, req, http.StatusForbidden)
	})
}
