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
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testCreateFile(t, session, "user2", "user2", "repo1", "master", "master", "direct", "test.txt", "Content", "")
		testCreateFile(
			t, session, "user2", "user2", "repo1", "master", "master", "direct", "test.txt", "Content",
			`A file named "test.txt" already exists in this repository.`)
		testCreateFile(t, session, "user2", "user2", "repo1", "master", "master", "commit-to-new-branch", "test2.txt", "Content",
			`Branch "master" already exists in this repository.`)
	})
}

func TestCreateFileFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")
		forkToEdit(t, session, "user2", "repo1", "_new", "master", "test.txt")
		testCreateFile(t, session, "user4", "user2", "repo1", "master", "feature/test", "commit-to-new-branch", "test.txt", "Content", "")
	})
}

func testCreateFile(t *testing.T, session *TestSession, user, owner, repo, branch, targetBranch, commitChoice, filePath, content, expectedError string) {
	// Request editor page
	newURL := fmt.Sprintf("/%s/%s/_new/%s/", owner, repo, branch)
	req := NewRequest(t, "GET", newURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req = NewRequestWithValues(t, "POST", newURL, map[string]string{
		"_csrf":           doc.GetCSRF(),
		"last_commit":     lastCommit,
		"tree_path":       filePath,
		"content":         content,
		"commit_choice":   commitChoice,
		"new_branch_name": targetBranch,
	})

	if expectedError != "" {
		resp := session.MakeRequest(t, req, http.StatusOK)

		// Check for expextecd error message
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t, htmlDoc.doc.Find(".ui.flash-message").Text(), expectedError)
		return
	}

	session.MakeRequest(t, req, http.StatusSeeOther)

	// Check new file exists
	req = NewRequestf(t, "GET", "/%s/%s/src/branch/%s/%s", user, repo, targetBranch, filePath)
	session.MakeRequest(t, req, http.StatusOK)
}

func TestCreateFileOnProtectedBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testCreateFileOnProtectedBranch(t, session, "user2", "user2", "repo1", "master", "master", "direct")
	})
}

func TestCreateFileOnProtectedBranchFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")
		forkToEdit(t, session, "user2", "repo1", "_new", "master", "test.txt")
		testCreateFileOnProtectedBranch(t, session, "user4", "user2", "repo1", "master", "feature/test", "commit-to-new-branch")
	})
}

func testCreateFileOnProtectedBranch(t *testing.T, session *TestSession, user, owner, repo, branch, targetBranch, commitChoice string) {
	csrf := GetUserCSRFToken(t, session)
	// Change target branch to protected
	req := NewRequestWithValues(t, "POST", path.Join(user, repo, "settings", "branches", "edit"), map[string]string{
		"_csrf":       csrf,
		"rule_name":   targetBranch,
		"enable_push": "true",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	// Check if target branch has been locked successfully
	flashMsg := session.GetCookieFlashMessage()
	assert.Equal(t, fmt.Sprintf(`Branch protection for rule "%s" has been updated.`, targetBranch), flashMsg.SuccessMsg)

	// Request editor page
	req = NewRequest(t, "GET", path.Join(owner, repo, "_new", branch))
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to target branch
	req = NewRequestWithValues(t, "POST", path.Join(owner, repo, "_new", branch), map[string]string{
		"_csrf":           doc.GetCSRF(),
		"last_commit":     lastCommit,
		"tree_path":       "test.txt",
		"content":         "Content",
		"commit_choice":   commitChoice,
		"new_branch_name": targetBranch,
	})

	resp = session.MakeRequest(t, req, http.StatusOK)
	// Check body for error message
	assert.Contains(t, resp.Body.String(), fmt.Sprintf("Cannot commit to protected branch &#34;%s&#34;.", targetBranch))

	// remove the protected branch
	csrf = GetUserCSRFToken(t, session)

	// Change target branch to protected
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "settings", "branches", "1", "delete"), map[string]string{
		"_csrf": csrf,
	})

	resp = session.MakeRequest(t, req, http.StatusOK)

	res := make(map[string]string)
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
	assert.Equal(t, "/"+path.Join(user, repo, "settings", "branches"), res["redirect"])

	// Check if target branch has been locked successfully
	flashMsg = session.GetCookieFlashMessage()
	assert.Equal(t, `Removing branch protection rule "1" failed.`, flashMsg.ErrorMsg)
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
	assert.Equal(t, newContent, resp.Body.String())

	return resp
}

func testEditFileToNewBranch(t *testing.T, session *TestSession, user, owner, repo, branch, targetBranch, filePath, newContent string) *httptest.ResponseRecorder {
	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(owner, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Submit the edits
	req = NewRequestWithValues(t, "POST", path.Join(owner, repo, "_edit", branch, filePath),
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
	assert.Equal(t, newContent, resp.Body.String())

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
		testEditFileToNewBranch(t, session, "user2", "user2", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited)\n")
	})
}

func TestEditFileToNewBranchFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")
		forkToEdit(t, session, "user2", "repo1", "_edit", "master", "README.md")
		testEditFileToNewBranch(t, session, "user4", "user2", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited)\n")
	})
}

func testEditFileDiffPreview(t *testing.T, session *TestSession, user, repo, branch, filePath string) {
	// Get to the 'edit this file' page
	req := NewRequest(t, "GET", path.Join(user, repo, "_edit", branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	lastCommit := htmlDoc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Preview the changes
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "_preview", branch, filePath),
		map[string]string{
			"_csrf":   htmlDoc.GetCSRF(),
			"content": "Hello, World (Edited)\n",
		},
	)
	resp = session.MakeRequest(t, req, http.StatusOK)

	assert.Contains(t, resp.Body.String(), `<span class="added-code">Hello, World (Edited)</span>`)
}

func TestEditFileDiffPreview(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testEditFileDiffPreview(t, session, "user2", "repo1", "master", "README.md")
	})
}

func TestDeleteFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testDeleteFile(t, session, "user2", "user2", "repo1", "master", "master", "direct", "README.md", "")
		testDeleteFile(t, session, "user2", "user2", "repo1", "master", "master", "direct", "MISSING.md",
			`The file being deleted, "MISSING.md", no longer exists in this repository.`)
	})
}

func TestDeleteFileFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")
		forkToEdit(t, session, "user2", "repo1", "_delete", "master", "README.md")
		testDeleteFile(t, session, "user4", "user2", "repo1", "master", "feature/test", "commit-to-new-branch", "README.md", "")
		testDeleteFile(t, session, "user4", "user2", "repo1", "master", "feature/missing", "commit-to-new-branch", "MISSING.md",
			`The file being deleted, "MISSING.md", no longer exists in this repository.`)
	})
}

func testDeleteFile(t *testing.T, session *TestSession, user, owner, repo, branch, targetBranch, commitChoice, filePath, expectedError string) {
	if expectedError == "" {
		// Check file exists
		req := NewRequestf(t, "GET", "/%s/%s/src/branch/%s/%s", owner, repo, branch, filePath)
		session.MakeRequest(t, req, http.StatusOK)
	}

	// Request editor page
	newURL := fmt.Sprintf("/%s/%s/_delete/%s/%s", owner, repo, branch, filePath)
	req := NewRequest(t, "GET", newURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save deleted file to target branch
	req = NewRequestWithValues(t, "POST", newURL, map[string]string{
		"_csrf":           doc.GetCSRF(),
		"last_commit":     lastCommit,
		"tree_path":       filePath,
		"commit_choice":   commitChoice,
		"new_branch_name": targetBranch,
	})

	if expectedError != "" {
		resp := session.MakeRequest(t, req, http.StatusOK)

		// Check for expextecd error message
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t, htmlDoc.doc.Find(".ui.flash-message").Text(), expectedError)
		return
	}

	session.MakeRequest(t, req, http.StatusSeeOther)

	// Check file was deleted
	req = NewRequestf(t, "GET", "/%s/%s/src/branch/%s/%s", user, repo, targetBranch, filePath)
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestPatchFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		testPatchFile(t, session, "user2", "user2", "repo1", "master", "feature/test", "Contents", "")
		testPatchFile(t, session, "user2", "user2", "repo1", "master", "feature/test", "Contents",
			`Branch "feature/test" already exists in this repository.`)
		testPatchFile(t, session, "user2", "user2", "repo1", "feature/test", "feature/again", "Conflict",
			`Unable to apply patch`)
	})
}

func TestPatchFileFork(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")
		forkToEdit(t, session, "user2", "repo1", "_diffpatch", "master", "README.md")
		testPatchFile(t, session, "user4", "user2", "repo1", "master", "feature/test", "Contents", "")
	})
}

func testPatchFile(t *testing.T, session *TestSession, user, owner, repo, branch, targetBranch, contents, expectedError string) {
	// Request editor page
	newURL := fmt.Sprintf("/%s/%s/_diffpatch/%s/", owner, repo, branch)
	req := NewRequest(t, "GET", newURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	lastCommit := doc.GetInputValueByName("last_commit")
	assert.NotEmpty(t, lastCommit)

	// Save new file to master branch
	req = NewRequestWithValues(t, "POST", newURL, map[string]string{
		"_csrf":       doc.GetCSRF(),
		"last_commit": lastCommit,
		"tree_path":   "__dummy__",
		"content": fmt.Sprintf(`diff --git a/patch-file-1.txt b/patch-file-1.txt
new file mode 100644
index 0000000000..aaaaaaaaaa
--- /dev/null
+++ b/patch-file-1.txt
@@ -0,0 +1 @@
+%s
`, contents),
		"commit_choice":   "commit-to-new-branch",
		"new_branch_name": targetBranch,
	})

	if expectedError != "" {
		resp := session.MakeRequest(t, req, http.StatusOK)

		// Check for expextecd error message
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Contains(t, htmlDoc.doc.Find(".ui.flash-message").Text(), expectedError)
		return
	}

	session.MakeRequest(t, req, http.StatusSeeOther)

	// Check new file exists
	req = NewRequestf(t, "GET", "/%s/%s/src/branch/%s/%s", user, repo, targetBranch, "patch-file-1.txt")
	session.MakeRequest(t, req, http.StatusOK)
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
				assert.Equal(t, expectedUserName, newCommit.Author.Name)
				assert.Equal(t, expectedEmail, newCommit.Author.Email)
				assert.Equal(t, expectedUserName, newCommit.Committer.Name)
				assert.Equal(t, expectedEmail, newCommit.Committer.Email)
			}
			return resp
		}

		uploadFile := func(t *testing.T, name, content string) string {
			body := &bytes.Buffer{}
			uploadForm := multipart.NewWriter(body)
			file, _ := uploadForm.CreateFormFile("file", name)
			_, _ = io.Copy(file, strings.NewReader(content))
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
			require.NotEqual(t, email.UID, user.ID)
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
			assert.Equal(t, "/user2/repo1/src/branch/master", resp1.Header().Get("Location"))
		})
	})
}

func forkToEdit(t *testing.T, session *TestSession, owner, repo, operation, branch, filePath string) {
	// Attempt to edit file
	req := NewRequest(t, "GET", path.Join(owner, repo, operation, branch, filePath))
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	// Fork
	req = NewRequestWithValues(t, "POST", path.Join(owner, repo, "_fork_to_edit", branch),
		map[string]string{
			"_csrf":          htmlDoc.GetCSRF(),
			"tree_path":      filePath,
			"edit_operation": operation,
		},
	)
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/"+path.Join(owner, repo, operation, branch, filePath), test.RedirectURL(resp))
}

func testForkToEditFile(t *testing.T, session *TestSession, user, owner, repo, branch, filePath string) {
	// Fork repository because we can't edit it
	forkToEdit(t, session, owner, repo, "_edit", branch, filePath)

	// Check the existence of the forked repo
	req := NewRequestf(t, "GET", "/%s/%s/settings", user, repo)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	// Archive the repository
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "settings"),
		map[string]string{
			"_csrf":     htmlDoc.GetCSRF(),
			"repo_name": repo,
			"action":    "archive",
		},
	)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Check editing archived repository is disabled
	req = NewRequest(t, "GET", path.Join(owner, repo, "_edit", branch, filePath))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "Fork Repository Not Editable")

	// Unfork the repository
	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "settings"),
		map[string]string{
			"_csrf":     htmlDoc.GetCSRF(),
			"repo_name": repo,
			"action":    "convert_fork",
		},
	)
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Fork repository again
	forkToEdit(t, session, owner, repo, "_edit", branch, filePath)

	// Check the existence of the forked repo with unique name
	req = NewRequestf(t, "GET", "/%s/%s-1", user, repo)
	session.MakeRequest(t, req, http.StatusOK)
}

func TestForkToEditFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")
		testForkToEditFile(t, session, "user4", "user2", "repo1", "master", "README.md")
	})
}

func TestEditFileNotAllowed(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user4")

		operations := []string{"_new", "_edit", "_delete", "_upload", "_diffpatch", "_cherrypick"}

		for _, operation := range operations {
			// Branch does not exist
			url := path.Join("user2", "repo1", operation, "missing", "README.md")
			req := NewRequest(t, "GET", url)
			session.MakeRequest(t, req, http.StatusNotFound)

			// Private repository
			url = path.Join("user2", "repo2", operation, "master", "Home.md")
			req = NewRequest(t, "GET", url)
			session.MakeRequest(t, req, http.StatusNotFound)

			// Empty repository
			url = path.Join("org41", "repo61", operation, "master", "README.md")
			req = NewRequest(t, "GET", url)
			session.MakeRequest(t, req, http.StatusNotFound)
		}
	})
}
