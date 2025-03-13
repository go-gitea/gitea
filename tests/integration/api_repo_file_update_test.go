// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"

	"github.com/stretchr/testify/assert"
)

func getUpdateFileOptions() *api.UpdateFileOptions {
	content := "This is updated text"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
	return &api.UpdateFileOptions{
		DeleteFileOptions: api.DeleteFileOptions{
			FileOptions: api.FileOptions{
				BranchName:    "master",
				NewBranchName: "master",
				Message:       "My update of new/file.txt",
				Author: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
				Committer: api.Identity{
					Name:  "Anne Doe",
					Email: "annedoe@example.com",
				},
			},
			SHA: "103ff9234cefeee5ec5361d22b49fbb04d385885",
		},
		ContentBase64: contentEncoded,
	}
}

func getExpectedFileResponseForUpdate(commitID, treePath, lastCommitSHA string) *api.FileResponse {
	sha := "08bd14b2e2852529157324de9c226b3364e76136"
	encoding := "base64"
	content := "VGhpcyBpcyB1cGRhdGVkIHRleHQ="
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=master"
	htmlURL := setting.AppURL + "user2/repo1/src/branch/master/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha
	downloadURL := setting.AppURL + "user2/repo1/raw/branch/master/" + treePath
	return &api.FileResponse{
		Content: &api.ContentsResponse{
			Name:          filepath.Base(treePath),
			Path:          treePath,
			SHA:           sha,
			LastCommitSHA: lastCommitSHA,
			Type:          "file",
			Size:          20,
			Encoding:      &encoding,
			Content:       &content,
			URL:           &selfURL,
			HTMLURL:       &htmlURL,
			GitURL:        &gitURL,
			DownloadURL:   &downloadURL,
			Links: &api.FileLinksResponse{
				Self:    &selfURL,
				GitURL:  &gitURL,
				HTMLURL: &htmlURL,
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: setting.AppURL + "user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Anne Doe",
					Email: "annedoe@example.com",
				},
			},
			Message: "My update of README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func TestAPIUpdateFile(t *testing.T) {
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
			req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &updateFileOptions).
				AddTokenAuth(token2)
			resp := MakeRequest(t, req, http.StatusOK)
			gitRepo, _ := gitrepo.OpenRepository(t.Context(), repo1)
			commitID, _ := gitRepo.GetBranchCommitID(updateFileOptions.NewBranchName)
			lasCommit, _ := gitRepo.GetCommitByPath(treePath)
			expectedFileResponse := getExpectedFileResponseForUpdate(commitID, treePath, lasCommit.ID.String())
			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)
			assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
			assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
			assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
			gitRepo.Close()
		}

		// Test updating a file in a new branch
		updateFileOptions := getUpdateFileOptions()
		updateFileOptions.BranchName = repo1.DefaultBranch
		updateFileOptions.NewBranchName = "new_branch"
		fileID++
		treePath := fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &updateFileOptions).
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusOK)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		expectedSHA := "08bd14b2e2852529157324de9c226b3364e76136"
		expectedHTMLURL := fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/new_branch/update/file%d.txt", fileID)
		expectedDownloadURL := fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/new_branch/update/file%d.txt", fileID)
		assert.EqualValues(t, expectedSHA, fileResponse.Content.SHA)
		assert.EqualValues(t, expectedHTMLURL, *fileResponse.Content.HTMLURL)
		assert.EqualValues(t, expectedDownloadURL, *fileResponse.Content.DownloadURL)
		assert.EqualValues(t, updateFileOptions.Message+"\n", fileResponse.Commit.Message)

		// Test updating a file and renaming it
		updateFileOptions = getUpdateFileOptions()
		updateFileOptions.BranchName = repo1.DefaultBranch
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		updateFileOptions.FromPath = treePath
		treePath = "rename/" + treePath
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &updateFileOptions).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &fileResponse)
		expectedSHA = "08bd14b2e2852529157324de9c226b3364e76136"
		expectedHTMLURL = fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/master/rename/update/file%d.txt", fileID)
		expectedDownloadURL = fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/master/rename/update/file%d.txt", fileID)
		assert.EqualValues(t, expectedSHA, fileResponse.Content.SHA)
		assert.EqualValues(t, expectedHTMLURL, *fileResponse.Content.HTMLURL)
		assert.EqualValues(t, expectedDownloadURL, *fileResponse.Content.DownloadURL)

		// Test updating a file without a message
		updateFileOptions = getUpdateFileOptions()
		updateFileOptions.Message = ""
		updateFileOptions.BranchName = repo1.DefaultBranch
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &updateFileOptions).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &fileResponse)
		expectedMessage := "Update " + treePath + "\n"
		assert.EqualValues(t, expectedMessage, fileResponse.Commit.Message)

		// Test updating a file with the wrong SHA
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		updateFileOptions = getUpdateFileOptions()
		correctSHA := updateFileOptions.SHA
		updateFileOptions.SHA = "badsha"
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &updateFileOptions).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusUnprocessableEntity)
		expectedAPIError := context.APIError{
			Message: "sha does not match [given: " + updateFileOptions.SHA + ", expected: " + correctSHA + "]",
			URL:     setting.API.SwaggerURL,
		}
		var apiError context.APIError
		DecodeJSON(t, resp, &apiError)
		assert.Equal(t, expectedAPIError, apiError)

		// Test creating a file in repo1 by user4 who does not have write access
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo16, treePath)
		updateFileOptions = getUpdateFileOptions()
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath), &updateFileOptions).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo16, treePath)
		updateFileOptions = getUpdateFileOptions()
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath), &updateFileOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo16, treePath)
		updateFileOptions = getUpdateFileOptions()
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath), &updateFileOptions).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)

		// Test using org repo "org3/repo3" where user2 is a collaborator
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(org3, repo3, treePath)
		updateFileOptions = getUpdateFileOptions()
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", org3.Name, repo3.Name, treePath), &updateFileOptions).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)

		// Test using org repo "org3/repo3" with no user token
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(org3, repo3, treePath)
		updateFileOptions = getUpdateFileOptions()
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", org3.Name, repo3.Name, treePath), &updateFileOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		fileID++
		treePath = fmt.Sprintf("update/file%d.txt", fileID)
		createFile(user2, repo1, treePath)
		updateFileOptions = getUpdateFileOptions()
		req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &updateFileOptions).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusForbidden)
	})
}
