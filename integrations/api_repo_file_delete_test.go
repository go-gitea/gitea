// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func getDeleteFileOptions() *api.DeleteFileOptions {
	return &api.DeleteFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "Removing the file new/file.txt",
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

func TestAPIDeleteFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
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

		// Test deleting file and making the delete in a new branch
		fileID++
		treePath := fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		deleteFileOptions := getDeleteFileOptions()
		deleteFileOptions.BranchName = repo1.DefaultBranch
		deleteFileOptions.NewBranchName = "new_branch"
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req := NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
		resp := session.MakeRequest(t, req, http.StatusOK)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		assert.NotNil(t, fileResponse)
		assert.Nil(t, fileResponse.Content)
		assert.EqualValues(t, deleteFileOptions.Message+"\n", fileResponse.Commit.Message)

		// Test deleting file without a message
		fileID++
		treePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		deleteFileOptions = getDeleteFileOptions()
		deleteFileOptions.Message = ""
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &fileResponse)
		expectedMessage := "Delete '" + treePath + "'\n"
		assert.EqualValues(t, expectedMessage, fileResponse.Commit.Message)

		// Test deleting a file with the wrong SHA
		fileID++
		treePath = fmt.Sprintf("delete/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		deleteFileOptions = getDeleteFileOptions()
		correctSHA := deleteFileOptions.SHA
		deleteFileOptions.SHA = "badsha"
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req = NewRequestWithJSON(t, "DELETE", url, &deleteFileOptions)
		resp = session.MakeRequest(t, req, http.StatusInternalServerError)
		expectedAPIError := context.APIError{
			Message: "sha does not match [given: " + deleteFileOptions.SHA + ", expected: " + correctSHA + "]",
			URL:     setting.API.SwaggerURL,
		}
		var apiError context.APIError
		DecodeJSON(t, resp, &apiError)
		assert.Equal(t, expectedAPIError, apiError)

		// Test creating a file in repo16 by user4 who does not have write access
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
	})
}
