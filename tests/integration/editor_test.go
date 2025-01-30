// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testCreateFile(t, session, "user2", "repo1", "master", "test.txt", "Content")
	})
}

func testCreateFile(t *testing.T, session *TestSession, user, repo, branch, filePath, content string) *httptest.ResponseRecorder {
	// Request editor page
	newURL := fmt.Sprintf("/%s/%s/_new/%s/", user, repo, branch)
	req := NewRequest(t, "GET", newURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req = NewRequestWithValues(t, "POST", newURL, map[string]string{
		"_csrf":         doc.GetCSRF(),
		"last_commit":   lastCommit,
		"tree_path":     filePath,
		"content":       content,
		"commit_choice": "direct",
	})
	return session.MakeRequest(t, req, http.StatusSeeOther)
}

func TestCreateFileOnProtectedBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")

		csrf := GetUserCSRFToken(t, session)
		// Change master branch to protected
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":       csrf,
			"rule_name":   "master",
			"enable_push": "true",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		// Check if master branch has been locked successfully
		flashMsg := session.GetCookieFlashMessage()
		assert.EqualValues(t, `Branch protection for rule "master" has been updated.`, flashMsg.SuccessMsg)

		// Request editor page
		req = NewRequest(t, "GET", "/user2/repo1/_new/master/")
		resp := session.MakeRequest(t, req, http.StatusOK)

		doc := NewHTMLParser(t, resp.Body)
		lastCommit := doc.GetInputValueByName("last_commit")
		assert.NotEmpty(t, lastCommit)

		// Save new file to master branch
		req = NewRequestWithValues(t, "POST", "/user2/repo1/_new/master/", map[string]string{
			"_csrf":         doc.GetCSRF(),
			"last_commit":   lastCommit,
			"tree_path":     "test.txt",
			"content":       "Content",
			"commit_choice": "direct",
		})

		resp = session.MakeRequest(t, req, http.StatusOK)
		// Check body for error message
		assert.Contains(t, resp.Body.String(), "Cannot commit to protected branch &#34;master&#34;.")

		// remove the protected branch
		csrf = GetUserCSRFToken(t, session)

		// Change master branch to protected
		req = NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/1/delete", map[string]string{
			"_csrf": csrf,
		})

		resp = session.MakeRequest(t, req, http.StatusOK)

		res := make(map[string]string)
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.EqualValues(t, "/user2/repo1/settings/branches", res["redirect"])

		// Check if master branch has been locked successfully
		flashMsg = session.GetCookieFlashMessage()
		assert.EqualValues(t, `Removing branch protection rule "1" failed.`, flashMsg.ErrorMsg)
	})
}

func testEditFile(t *testing.T, session *TestSession, user, repo, branch, filePath, newContent string) *httptest.ResponseRecorder {
	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(user, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Submit the edits
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "_edit", branch, filePath),
		map[string]string{
			"_csrf":         htmlDoc.GetCSRF(),
			"last_commit":   lastCommit,
			"tree_path":     filePath,
			"content":       newContent,
			"commit_choice": "direct",
		},
	)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw/branch", branch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, newContent, resp.Body.String())

	return resp
}

func testEditFileToNewBranch(t *testing.T, session *TestSession, user, repo, branch, targetBranch, filePath, newContent string) *httptest.ResponseRecorder {
	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(user, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Submit the edits
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "_edit", branch, filePath),
		map[string]string{
			"_csrf":           htmlDoc.GetCSRF(),
			"last_commit":     lastCommit,
			"tree_path":       filePath,
			"content":         newContent,
			"commit_choice":   "commit-to-new-branch",
			"new_branch_name": targetBranch,
		},
	)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify the change
	req = NewRequest(t, "GET", path.Join(user, repo, "raw/branch", targetBranch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, newContent, resp.Body.String())

	return resp
}

func TestEditFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testEditFile(t, session, "user2", "repo1", "master", "README.md", "Hello, World (Edited)\n")
	})
}

func TestEditFileToNewBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testEditFileToNewBranch(t, session, "user2", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited)\n")
	})
}

func TestEditFileCommitEmail(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		assert.True(t, user.KeepEmailPrivate)

		session := loginUser(t, user.Name)
		link := "/user2/repo1/_edit/master/README.md"

		getLastCommitID := func(t *testing.T) string {
			req := NewRequest(t, "GET", link)
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			lastCommit := htmlDoc.GetInputValueByName("last_commit")
			require.NotEmpty(t, lastCommit)
			return lastCommit
		}

		newReq := func(t *testing.T, session *TestSession, email, content string) *RequestWrapper {
			req := NewRequestWithValues(t, "POST", link, map[string]string{
				"_csrf":         GetUserCSRFToken(t, session),
				"last_commit":   getLastCommitID(t),
				"tree_path":     "README.md",
				"content":       content,
				"commit_choice": "direct",
				"commit_email":  email,
			})
			return req
		}

		t.Run("EmailInactive", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			email := unittest.AssertExistsAndLoadBean(t, &user_model.EmailAddress{ID: 35, UID: user.ID})
			assert.False(t, email.IsActivated)

			req := newReq(t, session, email.Email, "test content")
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.Contains(t,
				htmlDoc.doc.Find(".ui.negative.message").Text(),
				translation.NewLocale("en-US").Tr("repo.editor.invalid_commit_email"),
			)
		})

		t.Run("EmailInvalid", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			email := unittest.AssertExistsAndLoadBean(t, &user_model.EmailAddress{ID: 1, IsActivated: true})
			assert.NotEqualValues(t, email.UID, user.ID)

			req := newReq(t, session, email.Email, "test content")
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.Contains(t,
				htmlDoc.doc.Find(".ui.negative.message").Text(),
				translation.NewLocale("en-US").Tr("repo.editor.invalid_commit_email"),
			)
		})

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		gitRepo, _ := git.OpenRepository(git.DefaultContext, repo1.RepoPath())
		defer gitRepo.Close()

		t.Run("DefaultEmailKeepPrivate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := newReq(t, session, "", "privacy email")
			session.MakeRequest(t, req, http.StatusSeeOther)

			commit, err := gitRepo.GetCommitByPath("README.md")
			assert.NoError(t, err)

			fileContent, err := commit.GetFileContent("README.md", 64)
			assert.NoError(t, err)
			assert.EqualValues(t, "privacy email", fileContent)
			assert.EqualValues(t, "User Two", commit.Author.Name)
			assert.EqualValues(t, "user2@noreply.example.org", commit.Author.Email)
			assert.EqualValues(t, "User Two", commit.Committer.Name)
			assert.EqualValues(t, "user2@noreply.example.org", commit.Committer.Email)
		})

		t.Run("ChooseEmail", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			email := unittest.AssertExistsAndLoadBean(t, &user_model.EmailAddress{ID: 3, UID: user.ID, IsActivated: true})
			req := newReq(t, session, email.Email, "chosen email")
			session.MakeRequest(t, req, http.StatusSeeOther)

			commit, err := gitRepo.GetCommitByPath("README.md")
			assert.NoError(t, err)

			fileContent, err := commit.GetFileContent("README.md", 64)
			assert.NoError(t, err)
			assert.EqualValues(t, "chosen email", fileContent)
			assert.EqualValues(t, "User Two", commit.Author.Name)
			assert.EqualValues(t, email.Email, commit.Author.Email)
			assert.EqualValues(t, "User Two", commit.Committer.Name)
			assert.EqualValues(t, email.Email, commit.Committer.Email)
		})
	})
}
