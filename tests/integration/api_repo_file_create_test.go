// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	stdCtx "context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

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

func getCreateFileOptions() api.CreateFileOptions {
	content := "This is new text"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
	return api.CreateFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "Making this new file new/file.txt",
			Author: api.Identity{
				Name:  "Anne Doe",
				Email: "annedoe@example.com",
			},
			Committer: api.Identity{
				Name:  "John Doe",
				Email: "johndoe@example.com",
			},
			Dates: api.CommitDateOptions{
				Author:    time.Unix(946684810, 0),
				Committer: time.Unix(978307190, 0),
			},
		},
		Content: contentEncoded,
	}
}

func getExpectedFileResponseForCreate(repoFullName, commitID, treePath, latestCommitSHA string) *api.FileResponse {
	sha := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
	encoding := "base64"
	content := "VGhpcyBpcyBuZXcgdGV4dA=="
	selfURL := setting.AppURL + "api/v1/repos/" + repoFullName + "/contents/" + treePath + "?ref=master"
	htmlURL := setting.AppURL + repoFullName + "/src/branch/master/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/" + repoFullName + "/git/blobs/" + sha
	downloadURL := setting.AppURL + repoFullName + "/raw/branch/master/" + treePath
	return &api.FileResponse{
		Content: &api.ContentsResponse{
			Name:          filepath.Base(treePath),
			Path:          treePath,
			SHA:           sha,
			LastCommitSHA: latestCommitSHA,
			Size:          16,
			Type:          "file",
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
				URL: setting.AppURL + "api/v1/repos/" + repoFullName + "/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: setting.AppURL + repoFullName + "/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Anne Doe",
					Email: "annedoe@example.com",
				},
				Date: "2000-01-01T00:00:10Z",
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
				Date: "2000-12-31T23:59:50Z",
			},
			Message: "Updates README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func BenchmarkAPICreateFileSmall(b *testing.B) {
	onGiteaRunTB(b, func(t testing.TB, u *url.URL) {
		b := t.(*testing.B)
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})       // owner of the repo1 & repo16
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}) // public repo

		for n := 0; n < b.N; n++ {
			treePath := fmt.Sprintf("update/file%d.txt", n)
			createFileInBranch(user2, repo1, treePath, repo1.DefaultBranch, treePath)
		}
	})
}

func BenchmarkAPICreateFileMedium(b *testing.B) {
	data := make([]byte, 10*1024*1024)

	onGiteaRunTB(b, func(t testing.TB, u *url.URL) {
		b := t.(*testing.B)
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})       // owner of the repo1 & repo16
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}) // public repo

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			treePath := fmt.Sprintf("update/file%d.txt", n)
			copy(data, treePath)
			createFileInBranch(user2, repo1, treePath, repo1.DefaultBranch, treePath)
		}
	})
}

func TestAPICreateFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
		user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})         // owner of the repo3, is an org
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
		repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo
		fileID := 0

		// Get user2's token
		session := loginUser(t, user2.Name)
		token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		// Get user4's token
		session = loginUser(t, user4.Name)
		token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

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
			resp := MakeRequest(t, req, http.StatusCreated)
			gitRepo, _ := git.OpenRepository(stdCtx.Background(), repo1.RepoPath())
			commitID, _ := gitRepo.GetBranchCommitID(createFileOptions.NewBranchName)
			latestCommit, _ := gitRepo.GetCommitByPath(treePath)
			expectedFileResponse := getExpectedFileResponseForCreate("user2/repo1", commitID, treePath, latestCommit.ID.String())
			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)
			assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
			assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
			assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Date, fileResponse.Commit.Author.Date)
			assert.EqualValues(t, expectedFileResponse.Commit.Committer.Email, fileResponse.Commit.Committer.Email)
			assert.EqualValues(t, expectedFileResponse.Commit.Committer.Name, fileResponse.Commit.Committer.Name)
			assert.EqualValues(t, expectedFileResponse.Commit.Committer.Date, fileResponse.Commit.Committer.Date)
			gitRepo.Close()
		}

		// Test creating a file in a new branch
		createFileOptions := getCreateFileOptions()
		createFileOptions.BranchName = repo1.DefaultBranch
		createFileOptions.NewBranchName = "new_branch"
		fileID++
		treePath := fmt.Sprintf("new/file%d.txt", fileID)
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req := NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp := MakeRequest(t, req, http.StatusCreated)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		expectedSHA := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
		expectedHTMLURL := fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/new_branch/new/file%d.txt", fileID)
		expectedDownloadURL := fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/new_branch/new/file%d.txt", fileID)
		assert.EqualValues(t, expectedSHA, fileResponse.Content.SHA)
		assert.EqualValues(t, expectedHTMLURL, *fileResponse.Content.HTMLURL)
		assert.EqualValues(t, expectedDownloadURL, *fileResponse.Content.DownloadURL)
		assert.EqualValues(t, createFileOptions.Message+"\n", fileResponse.Commit.Message)

		// Test creating a file without a message
		createFileOptions = getCreateFileOptions()
		createFileOptions.Message = ""
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp = MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &fileResponse)
		expectedMessage := "Add " + treePath + "\n"
		assert.EqualValues(t, expectedMessage, fileResponse.Commit.Message)

		// Test trying to create a file that already exists, should fail
		createFileOptions = getCreateFileOptions()
		treePath = "README.md"
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp = MakeRequest(t, req, http.StatusUnprocessableEntity)
		expectedAPIError := context.APIError{
			Message: "repository file already exists [path: " + treePath + "]",
			URL:     setting.API.SwaggerURL,
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
		MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "user3/repo3" where user2 is a collaborator
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "user3/repo3" with no user token
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user3.Name, repo3.Name, treePath)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token4)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		MakeRequest(t, req, http.StatusForbidden)

		// Test creating a file in an empty repository
		doAPICreateRepository(NewAPITestContext(t, "user2", "empty-repo", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser), true)(t)
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, "empty-repo", treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp = MakeRequest(t, req, http.StatusCreated)
		emptyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "empty-repo"}) // public repo
		gitRepo, _ := git.OpenRepository(stdCtx.Background(), emptyRepo.RepoPath())
		commitID, _ := gitRepo.GetBranchCommitID(createFileOptions.NewBranchName)
		latestCommit, _ := gitRepo.GetCommitByPath(treePath)
		expectedFileResponse := getExpectedFileResponseForCreate("user2/empty-repo", commitID, treePath, latestCommit.ID.String())
		DecodeJSON(t, resp, &fileResponse)
		assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
		assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
		assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
		assert.EqualValues(t, expectedFileResponse.Commit.Author.Date, fileResponse.Commit.Author.Date)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Email, fileResponse.Commit.Committer.Email)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Name, fileResponse.Commit.Committer.Name)
		assert.EqualValues(t, expectedFileResponse.Commit.Committer.Date, fileResponse.Commit.Committer.Date)
		gitRepo.Close()
	})
}
