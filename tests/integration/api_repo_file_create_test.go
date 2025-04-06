// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

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
		ContentBase64: contentEncoded,
	}
}

func normalizeFileContentResponseCommitTime(c *api.ContentsResponse) {
	// decoded JSON response may contain different timezone from the one parsed by git commit
	// so we need to normalize the time to UTC to make "assert.Equal" pass
	c.LastCommitterDate = c.LastCommitterDate.UTC()
	c.LastAuthorDate = c.LastAuthorDate.UTC()
}

type apiFileResponseInfo struct {
	repoFullName, commitID, treePath, lastCommitSHA string
	lastCommitterWhen, lastAuthorWhen               time.Time
}

func getExpectedFileResponseForCreate(info apiFileResponseInfo) *api.FileResponse {
	sha := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
	encoding := "base64"
	content := "VGhpcyBpcyBuZXcgdGV4dA=="
	selfURL := setting.AppURL + "api/v1/repos/" + info.repoFullName + "/contents/" + info.treePath + "?ref=master"
	htmlURL := setting.AppURL + info.repoFullName + "/src/branch/master/" + info.treePath
	gitURL := setting.AppURL + "api/v1/repos/" + info.repoFullName + "/git/blobs/" + sha
	downloadURL := setting.AppURL + info.repoFullName + "/raw/branch/master/" + info.treePath
	ret := &api.FileResponse{
		Content: &api.ContentsResponse{
			Name:              path.Base(info.treePath),
			Path:              info.treePath,
			SHA:               sha,
			LastCommitSHA:     info.lastCommitSHA,
			LastCommitterDate: info.lastCommitterWhen,
			LastAuthorDate:    info.lastAuthorWhen,
			Size:              16,
			Type:              "file",
			Encoding:          &encoding,
			Content:           &content,
			URL:               &selfURL,
			HTMLURL:           &htmlURL,
			GitURL:            &gitURL,
			DownloadURL:       &downloadURL,
			Links: &api.FileLinksResponse{
				Self:    &selfURL,
				GitURL:  &gitURL,
				HTMLURL: &htmlURL,
			},
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/" + info.repoFullName + "/git/commits/" + info.commitID,
				SHA: info.commitID,
			},
			HTMLURL: setting.AppURL + info.repoFullName + "/commit/" + info.commitID,
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
	normalizeFileContentResponseCommitTime(ret.Content)
	return ret
}

func BenchmarkAPICreateFileSmall(b *testing.B) {
	onGiteaRun(b, func(b *testing.B, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(b, &user_model.User{ID: 2})       // owner of the repo1 & repo16
		repo1 := unittest.AssertExistsAndLoadBean(b, &repo_model.Repository{ID: 1}) // public repo

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			treePath := fmt.Sprintf("update/file%d.txt", n)
			_, _ = createFileInBranch(user2, repo1, treePath, repo1.DefaultBranch, treePath)
		}
	})
}

func BenchmarkAPICreateFileMedium(b *testing.B) {
	data := make([]byte, 10*1024*1024)

	onGiteaRun(b, func(b *testing.B, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(b, &user_model.User{ID: 2})       // owner of the repo1 & repo16
		repo1 := unittest.AssertExistsAndLoadBean(b, &repo_model.Repository{ID: 1}) // public repo

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			treePath := fmt.Sprintf("update/file%d.txt", n)
			copy(data, treePath)
			_, _ = createFileInBranch(user2, repo1, treePath, repo1.DefaultBranch, treePath)
		}
	})
}

func TestAPICreateFile(t *testing.T) {
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
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &createFileOptions).
				AddTokenAuth(token2)
			resp := MakeRequest(t, req, http.StatusCreated)
			gitRepo, _ := gitrepo.OpenRepository(t.Context(), repo1)
			defer gitRepo.Close()
			commitID, _ := gitRepo.GetBranchCommitID(createFileOptions.NewBranchName)
			lastCommit, _ := gitRepo.GetCommitByPath(treePath)
			expectedFileResponse := getExpectedFileResponseForCreate(apiFileResponseInfo{
				repoFullName:      "user2/repo1",
				commitID:          commitID,
				treePath:          treePath,
				lastCommitSHA:     lastCommit.ID.String(),
				lastCommitterWhen: lastCommit.Committer.When,
				lastAuthorWhen:    lastCommit.Author.When,
			})
			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)
			normalizeFileContentResponseCommitTime(fileResponse.Content)
			assert.Equal(t, expectedFileResponse.Content, fileResponse.Content)
			assert.Equal(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
			assert.Equal(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
			assert.Equal(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
			assert.Equal(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
			assert.Equal(t, expectedFileResponse.Commit.Author.Date, fileResponse.Commit.Author.Date)
			assert.Equal(t, expectedFileResponse.Commit.Committer.Email, fileResponse.Commit.Committer.Email)
			assert.Equal(t, expectedFileResponse.Commit.Committer.Name, fileResponse.Commit.Committer.Name)
			assert.Equal(t, expectedFileResponse.Commit.Committer.Date, fileResponse.Commit.Committer.Date)
		}

		// Test creating a file in a new branch
		createFileOptions := getCreateFileOptions()
		createFileOptions.BranchName = repo1.DefaultBranch
		createFileOptions.NewBranchName = "new_branch"
		fileID++
		treePath := fmt.Sprintf("new/file%d.txt", fileID)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &createFileOptions).
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusCreated)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		expectedSHA := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
		expectedHTMLURL := fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/new_branch/new/file%d.txt", fileID)
		expectedDownloadURL := fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/new_branch/new/file%d.txt", fileID)
		assert.Equal(t, expectedSHA, fileResponse.Content.SHA)
		assert.Equal(t, expectedHTMLURL, *fileResponse.Content.HTMLURL)
		assert.Equal(t, expectedDownloadURL, *fileResponse.Content.DownloadURL)
		assert.Equal(t, createFileOptions.Message+"\n", fileResponse.Commit.Message)

		// Test creating a file without a message
		createFileOptions = getCreateFileOptions()
		createFileOptions.Message = ""
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &createFileOptions).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &fileResponse)
		expectedMessage := "Add " + treePath + "\n"
		assert.Equal(t, expectedMessage, fileResponse.Commit.Message)

		// Test trying to create a file that already exists, should fail
		createFileOptions = getCreateFileOptions()
		treePath = "README.md"
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &createFileOptions).
			AddTokenAuth(token2)
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
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath), &createFileOptions).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath), &createFileOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath), &createFileOptions).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "org3/repo3" where user2 is a collaborator
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", org3.Name, repo3.Name, treePath), &createFileOptions).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "org3/repo3" with no user token
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", org3.Name, repo3.Name, treePath), &createFileOptions)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath), &createFileOptions).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusForbidden)

		// Test creating a file in an empty repository
		doAPICreateRepository(NewAPITestContext(t, "user2", "empty-repo", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser), true)(t)
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, "empty-repo", treePath), &createFileOptions).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusCreated)
		emptyRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "empty-repo"}) // public repo
		gitRepo, _ := gitrepo.OpenRepository(t.Context(), emptyRepo)
		defer gitRepo.Close()
		commitID, _ := gitRepo.GetBranchCommitID(createFileOptions.NewBranchName)
		latestCommit, _ := gitRepo.GetCommitByPath(treePath)
		expectedFileResponse := getExpectedFileResponseForCreate(apiFileResponseInfo{
			repoFullName:      "user2/empty-repo",
			commitID:          commitID,
			treePath:          treePath,
			lastCommitSHA:     latestCommit.ID.String(),
			lastCommitterWhen: latestCommit.Committer.When,
			lastAuthorWhen:    latestCommit.Author.When,
		})
		DecodeJSON(t, resp, &fileResponse)
		normalizeFileContentResponseCommitTime(fileResponse.Content)
		assert.Equal(t, expectedFileResponse.Content, fileResponse.Content)
		assert.Equal(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
		assert.Equal(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
		assert.Equal(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
		assert.Equal(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
		assert.Equal(t, expectedFileResponse.Commit.Author.Date, fileResponse.Commit.Author.Date)
		assert.Equal(t, expectedFileResponse.Commit.Committer.Email, fileResponse.Commit.Committer.Email)
		assert.Equal(t, expectedFileResponse.Commit.Committer.Name, fileResponse.Commit.Committer.Name)
		assert.Equal(t, expectedFileResponse.Commit.Committer.Date, fileResponse.Commit.Committer.Date)
	})
}
