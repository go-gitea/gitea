// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
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

func TestWebGitCommitEmail(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		require.True(t, user.KeepEmailPrivate)

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		gitRepo, _ := git.OpenRepository(git.DefaultContext, repo1.RepoPath())
		defer gitRepo.Close()
		getLastCommit := func(t *testing.T) *git.Commit {
			c, err := gitRepo.GetBranchCommit("master")
			require.NoError(t, err)
			return c
		}

		session := loginUser(t, user.Name)

		makeReq := func(t *testing.T, link string, params map[string]string, expectedUserName, expectedEmail string) *httptest.ResponseRecorder {
			lastCommit := getLastCommit(t)
			params["_csrf"] = GetUserCSRFToken(t, session)
			params["last_commit"] = lastCommit.ID.String()
			params["commit_choice"] = "direct"
			req := NewRequestWithValues(t, "POST", link, params)
			resp := session.MakeRequest(t, req, NoExpectedStatus)
			newCommit := getLastCommit(t)
			if expectedUserName == "" {
				require.Equal(t, lastCommit.ID.String(), newCommit.ID.String())
				htmlDoc := NewHTMLParser(t, resp.Body)
				errMsg := htmlDoc.doc.Find(".ui.negative.message").Text()
				assert.Contains(t, errMsg, translation.NewLocale("en-US").Tr("repo.editor.invalid_commit_email"))
			} else {
				require.NotEqual(t, lastCommit.ID.String(), newCommit.ID.String())
				assert.EqualValues(t, expectedUserName, newCommit.Author.Name)
				assert.EqualValues(t, expectedEmail, newCommit.Author.Email)
				assert.EqualValues(t, expectedUserName, newCommit.Committer.Name)
				assert.EqualValues(t, expectedEmail, newCommit.Committer.Email)
			}
			return resp
		}

		uploadFile := func(t *testing.T, name, content string) string {
			body := &bytes.Buffer{}
			uploadForm := multipart.NewWriter(body)
			file, _ := uploadForm.CreateFormFile("file", name)
			_, _ = io.Copy(file, bytes.NewBufferString(content))
			_ = uploadForm.WriteField("_csrf", GetUserCSRFToken(t, session))
			_ = uploadForm.Close()

			req := NewRequestWithBody(t, "POST", "/user2/repo1/upload-file", body)
			req.Header.Add("Content-Type", uploadForm.FormDataContentType())
			resp := session.MakeRequest(t, req, http.StatusOK)

			respMap := map[string]string{}
			DecodeJSON(t, resp, &respMap)
			return respMap["uuid"]
		}

		t.Run("EmailInactive", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			email := unittest.AssertExistsAndLoadBean(t, &user_model.EmailAddress{ID: 35, UID: user.ID})
			require.False(t, email.IsActivated)
			makeReq(t, "/user2/repo1/_edit/master/README.md", map[string]string{
				"tree_path":    "README.md",
				"content":      "test content",
				"commit_email": email.Email,
			}, "", "")
		})

		t.Run("EmailInvalid", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			email := unittest.AssertExistsAndLoadBean(t, &user_model.EmailAddress{ID: 1, IsActivated: true})
			require.NotEqualValues(t, email.UID, user.ID)
			makeReq(t, "/user2/repo1/_edit/master/README.md", map[string]string{
				"tree_path":    "README.md",
				"content":      "test content",
				"commit_email": email.Email,
			}, "", "")
		})

		testWebGit := func(t *testing.T, linkForKeepPrivate string, paramsForKeepPrivate map[string]string, linkForChosenEmail string, paramsForChosenEmail map[string]string) (resp1, resp2 *httptest.ResponseRecorder) {
			t.Run("DefaultEmailKeepPrivate", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				paramsForKeepPrivate["commit_email"] = ""
				resp1 = makeReq(t, linkForKeepPrivate, paramsForKeepPrivate, "User Two", "user2@noreply.example.org")
			})
			t.Run("ChooseEmail", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				paramsForChosenEmail["commit_email"] = "user2@example.com"
				resp2 = makeReq(t, linkForChosenEmail, paramsForChosenEmail, "User Two", "user2@example.com")
			})
			return resp1, resp2
		}

		t.Run("Edit", func(t *testing.T) {
			testWebGit(t,
				"/user2/repo1/_edit/master/README.md", map[string]string{"tree_path": "README.md", "content": "for keep private"},
				"/user2/repo1/_edit/master/README.md", map[string]string{"tree_path": "README.md", "content": "for chosen email"},
			)
		})

		t.Run("UploadDelete", func(t *testing.T) {
			file1UUID := uploadFile(t, "file1", "File 1")
			file2UUID := uploadFile(t, "file2", "File 2")
			testWebGit(t,
				"/user2/repo1/_upload/master", map[string]string{"files": file1UUID},
				"/user2/repo1/_upload/master", map[string]string{"files": file2UUID},
			)
			testWebGit(t,
				"/user2/repo1/_delete/master/file1", map[string]string{},
				"/user2/repo1/_delete/master/file2", map[string]string{},
			)
		})

		t.Run("ApplyPatchCherryPick", func(t *testing.T) {
			testWebGit(t,
				"/user2/repo1/_diffpatch/master", map[string]string{
					"tree_path": "__dummy__",
					"content": `diff --git a/patch-file-1.txt b/patch-file-1.txt
new file mode 100644
index 0000000000..aaaaaaaaaa
--- /dev/null
+++ b/patch-file-1.txt
@@ -0,0 +1 @@
+File 1
`,
				},
				"/user2/repo1/_diffpatch/master", map[string]string{
					"tree_path": "__dummy__",
					"content": `diff --git a/patch-file-2.txt b/patch-file-2.txt
new file mode 100644
index 0000000000..bbbbbbbbbb
--- /dev/null
+++ b/patch-file-2.txt
@@ -0,0 +1 @@
+File 2
`,
				},
			)

			commit1, err := gitRepo.GetCommitByPath("patch-file-1.txt")
			require.NoError(t, err)
			commit2, err := gitRepo.GetCommitByPath("patch-file-2.txt")
			require.NoError(t, err)
			resp1, _ := testWebGit(t,
				"/user2/repo1/_cherrypick/"+commit1.ID.String()+"/master", map[string]string{"revert": "true"},
				"/user2/repo1/_cherrypick/"+commit2.ID.String()+"/master", map[string]string{"revert": "true"},
			)

			// By the way, test the "cherrypick" page: a successful revert redirects to the main branch
			assert.EqualValues(t, "/user2/repo1/src/branch/master", resp1.Header().Get("Location"))
		})
	})
}
