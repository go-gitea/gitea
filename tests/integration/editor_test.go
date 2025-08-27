// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"maps"
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
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditor(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		sessionUser2 := loginUser(t, "user2")
		t.Run("EditFileNotAllowed", testEditFileNotAllowed)
		t.Run("DiffPreview", testEditorDiffPreview)
		t.Run("CreateFile", testEditorCreateFile)
		t.Run("EditFile", func(t *testing.T) {
			testEditFile(t, sessionUser2, "user2", "repo1", "master", "README.md", "Hello, World (direct)\n")
			testEditFileToNewBranch(t, sessionUser2, "user2", "repo1", "master", "feature/test", "README.md", "Hello, World (commit-to-new-branch)\n")
		})
		t.Run("PatchFile", testEditorPatchFile)
		t.Run("DeleteFile", func(t *testing.T) {
			viewLink := "/user2/repo1/src/branch/branch2/README.md"
			sessionUser2.MakeRequest(t, NewRequest(t, "GET", viewLink), http.StatusOK)
			testEditorActionPostRequest(t, sessionUser2, "/user2/repo1/_delete/branch2/README.md", map[string]string{"commit_choice": "direct"})
			sessionUser2.MakeRequest(t, NewRequest(t, "GET", viewLink), http.StatusNotFound)
		})
		t.Run("ForkToEditFile", func(t *testing.T) {
			testForkToEditFile(t, loginUser(t, "user4"), "user4", "user2", "repo1", "master", "README.md")
		})
		t.Run("WebGitCommitEmail", testEditorWebGitCommitEmail)
		t.Run("ProtectedBranch", testEditorProtectedBranch)
	})
}

func testEditorCreateFile(t *testing.T) {
	session := loginUser(t, "user2")
	testCreateFile(t, session, "user2", "repo1", "master", "", "test.txt", "Content")
	testEditorActionPostRequestError(t, session, "/user2/repo1/_new/master/", map[string]string{
		"tree_path":       "test.txt",
		"commit_choice":   "direct",
		"new_branch_name": "master",
	}, `A file named "test.txt" already exists in this repository.`)
	testEditorActionPostRequestError(t, session, "/user2/repo1/_new/master/", map[string]string{
		"tree_path":       "test.txt",
		"commit_choice":   "commit-to-new-branch",
		"new_branch_name": "master",
	}, `Branch "master" already exists in this repository.`)
}

func testCreateFile(t *testing.T, session *TestSession, user, repo, baseBranchName, newBranchName, filePath, content string) {
	commitChoice := "direct"
	if newBranchName != "" && newBranchName != baseBranchName {
		commitChoice = "commit-to-new-branch"
	}
	testEditorActionEdit(t, session, user, repo, "_new", baseBranchName, "", map[string]string{
		"tree_path":       filePath,
		"content":         content,
		"commit_choice":   commitChoice,
		"new_branch_name": newBranchName,
	})
}

func testEditorProtectedBranch(t *testing.T) {
	session := loginUser(t, "user2")
	// Change the "master" branch to "protected"
	req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
		"_csrf":       GetUserCSRFToken(t, session),
		"rule_name":   "master",
		"enable_push": "true",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	flashMsg := session.GetCookieFlashMessage()
	assert.Equal(t, `Branch protection for rule "master" has been updated.`, flashMsg.SuccessMsg)

	// Try to commit a file to the "master" branch and it should fail
	resp := testEditorActionPostRequest(t, session, "/user2/repo1/_new/master/", map[string]string{"tree_path": "test-protected-branch.txt", "commit_choice": "direct"})
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, `Cannot commit to protected branch "master".`, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)
}

func testEditorActionPostRequest(t *testing.T, session *TestSession, requestPath string, params map[string]string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", requestPath)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	form := map[string]string{
		"_csrf":       htmlDoc.GetCSRF(),
		"last_commit": htmlDoc.GetInputValueByName("last_commit"),
	}
	maps.Copy(form, params)
	req = NewRequestWithValues(t, "POST", requestPath, form)
	return session.MakeRequest(t, req, NoExpectedStatus)
}

func testEditorActionPostRequestError(t *testing.T, session *TestSession, requestPath string, params map[string]string, errorMessage string) {
	resp := testEditorActionPostRequest(t, session, requestPath, params)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, errorMessage, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)
}

func testEditorActionEdit(t *testing.T, session *TestSession, user, repo, editorAction, branch, filePath string, params map[string]string) *httptest.ResponseRecorder {
	params["tree_path"] = util.IfZero(params["tree_path"], filePath)
	newBranchName := util.Iif(params["commit_choice"] == "direct", branch, params["new_branch_name"])
	resp := testEditorActionPostRequest(t, session, fmt.Sprintf("/%s/%s/%s/%s/%s", user, repo, editorAction, branch, filePath), params)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotEmpty(t, test.RedirectURL(resp))
	req := NewRequest(t, "GET", path.Join(user, repo, "raw/branch", newBranchName, params["tree_path"]))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, params["content"], resp.Body.String())
	return resp
}

func testEditFile(t *testing.T, session *TestSession, user, repo, branch, filePath, newContent string) {
	testEditorActionEdit(t, session, user, repo, "_edit", branch, filePath, map[string]string{
		"content":       newContent,
		"commit_choice": "direct",
	})
}

func testEditFileToNewBranch(t *testing.T, session *TestSession, user, repo, branch, targetBranch, filePath, newContent string) {
	testEditorActionEdit(t, session, user, repo, "_edit", branch, filePath, map[string]string{
		"content":         newContent,
		"commit_choice":   "commit-to-new-branch",
		"new_branch_name": targetBranch,
	})
}

func testEditorDiffPreview(t *testing.T) {
	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user2/repo1/_preview/master/README.md", map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": "Hello, World (Edited)\n",
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<span class="added-code">Hello, World (Edited)</span>`)
}

func testEditorPatchFile(t *testing.T) {
	session := loginUser(t, "user2")
	pathContentCommon := `diff --git a/patch-file-1.txt b/patch-file-1.txt
new file mode 100644
index 0000000000..aaaaaaaaaa
--- /dev/null
+++ b/patch-file-1.txt
@@ -0,0 +1 @@
+`
	testEditorActionPostRequest(t, session, "/user2/repo1/_diffpatch/master/", map[string]string{
		"content":         pathContentCommon + "patched content\n",
		"commit_choice":   "commit-to-new-branch",
		"new_branch_name": "patched-branch",
	})
	resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/raw/branch/patched-branch/patch-file-1.txt"), http.StatusOK)
	assert.Equal(t, "patched content\n", resp.Body.String())

	// patch again, it should fail
	resp = testEditorActionPostRequest(t, session, "/user2/repo1/_diffpatch/patched-branch/", map[string]string{
		"content":         pathContentCommon + "another patched content\n",
		"commit_choice":   "commit-to-new-branch",
		"new_branch_name": "patched-branch-1",
	})
	assert.Equal(t, "Unable to apply patch", test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)
}

func testEditorWebGitCommitEmail(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.True(t, user.KeepEmailPrivate)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, _ := git.OpenRepository(t.Context(), repo1.RepoPath())
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
			respErr := test.ParseJSONError(resp.Body.Bytes())
			assert.Equal(t, translation.NewLocale("en-US").TrString("repo.editor.invalid_commit_email"), respErr.ErrorMessage)
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
		assert.Equal(t, "/user2/repo1/src/branch/master", test.RedirectURL(resp1))
	})
}

func testForkToEditFile(t *testing.T, session *TestSession, user, owner, repo, branch, filePath string) {
	forkToEdit := func(t *testing.T, session *TestSession, owner, repo, operation, branch, filePath string) {
		// visit the base repo, see the "Add File" button
		req := NewRequest(t, "GET", path.Join(owner, repo))
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, htmlDoc, ".repo-add-file", 1)

		// attempt to edit a file, see the guideline page
		req = NewRequest(t, "GET", path.Join(owner, repo, operation, branch, filePath))
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), "Fork Repository to Propose Changes")

		// fork the repository
		req = NewRequestWithValues(t, "POST", path.Join(owner, repo, "_fork", branch), map[string]string{"_csrf": GetUserCSRFToken(t, session)})
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.JSONEq(t, `{"redirect":""}`, resp.Body.String())
	}

	t.Run("ForkButArchived", func(t *testing.T) {
		// Fork repository because we can't edit it
		forkToEdit(t, session, owner, repo, "_edit", branch, filePath)

		// Archive the repository
		req := NewRequestWithValues(t, "POST", path.Join(user, repo, "settings"),
			map[string]string{
				"_csrf":     GetUserCSRFToken(t, session),
				"repo_name": repo,
				"action":    "archive",
			},
		)
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Check editing archived repository is disabled
		req = NewRequest(t, "GET", path.Join(owner, repo, "_edit", branch, filePath)).SetHeader("Accept", "text/html")
		resp := session.MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), "You have forked this repository but your fork is not editable.")

		// Unfork the repository
		req = NewRequestWithValues(t, "POST", path.Join(user, repo, "settings"),
			map[string]string{
				"_csrf":     GetUserCSRFToken(t, session),
				"repo_name": repo,
				"action":    "convert_fork",
			},
		)
		session.MakeRequest(t, req, http.StatusSeeOther)
	})

	// Fork repository again, and check the existence of the forked repo with unique name
	forkToEdit(t, session, owner, repo, "_edit", branch, filePath)
	session.MakeRequest(t, NewRequestf(t, "GET", "/%s/%s-1", user, repo), http.StatusOK)

	t.Run("CheckBaseRepoForm", func(t *testing.T) {
		// the base repo's edit form should have the correct action and upload links (pointing to the forked repo)
		req := NewRequest(t, "GET", path.Join(owner, repo, "_upload", branch, filePath)+"?foo=bar")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		uploadForm := htmlDoc.doc.Find(".form-fetch-action")
		formAction := uploadForm.AttrOr("action", "")
		assert.Equal(t, fmt.Sprintf("/%s/%s-1/_upload/%s/%s?from_base_branch=%s&foo=bar", user, repo, branch, filePath, branch), formAction)
		uploadLink := uploadForm.Find(".dropzone").AttrOr("data-link-url", "")
		assert.Equal(t, fmt.Sprintf("/%s/%s-1/upload-file", user, repo), uploadLink)
		newBranchName := uploadForm.Find("input[name=new_branch_name]").AttrOr("value", "")
		assert.Equal(t, user+"-patch-1", newBranchName)
		commitChoice := uploadForm.Find("input[name=commit_choice][checked]").AttrOr("value", "")
		assert.Equal(t, "commit-to-new-branch", commitChoice)
		lastCommit := uploadForm.Find("input[name=last_commit]").AttrOr("value", "")
		assert.NotEmpty(t, lastCommit)
	})

	t.Run("ViewBaseEditFormAndCommitToFork", func(t *testing.T) {
		req := NewRequest(t, "GET", path.Join(owner, repo, "_edit", branch, filePath))
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		editRequestForm := map[string]string{
			"_csrf":         GetUserCSRFToken(t, session),
			"last_commit":   htmlDoc.GetInputValueByName("last_commit"),
			"tree_path":     filePath,
			"content":       "new content in fork",
			"commit_choice": "commit-to-new-branch",
		}
		// change a file in the forked repo with existing branch name (should fail)
		editRequestForm["new_branch_name"] = "master"
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s-1/_edit/%s/%s?from_base_branch=%s", user, repo, branch, filePath, branch), editRequestForm)
		resp = session.MakeRequest(t, req, http.StatusBadRequest)
		respJSON := test.ParseJSONError(resp.Body.Bytes())
		assert.Equal(t, `Branch "master" already exists in your fork. Please choose a new branch name.`, respJSON.ErrorMessage)

		// change a file in the forked repo (should succeed)
		newBranchName := htmlDoc.GetInputValueByName("new_branch_name")
		editRequestForm["new_branch_name"] = newBranchName
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s-1/_edit/%s/%s?from_base_branch=%s", user, repo, branch, filePath, branch), editRequestForm)
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, fmt.Sprintf("/%s/%s/compare/%s...%s/%s-1:%s", owner, repo, branch, user, repo, newBranchName), test.RedirectURL(resp))

		// check the file in the fork's branch is changed
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s-1/src/branch/%s/%s", user, repo, newBranchName, filePath))
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), "new content in fork")
	})
}

func testEditFileNotAllowed(t *testing.T) {
	sessionUser1 := loginUser(t, "user1") // admin, all access
	sessionUser4 := loginUser(t, "user4")
	// "_cherrypick" has a different route pattern, so skip its test
	operations := []string{"_new", "_edit", "_delete", "_upload", "_diffpatch"}
	for _, operation := range operations {
		t.Run(operation, func(t *testing.T) {
			// Branch does not exist
			targetLink := path.Join("user2", "repo1", operation, "missing", "README.md")
			sessionUser1.MakeRequest(t, NewRequest(t, "GET", targetLink), http.StatusNotFound)

			// Private repository
			targetLink = path.Join("user2", "repo2", operation, "master", "Home.md")
			sessionUser1.MakeRequest(t, NewRequest(t, "GET", targetLink), http.StatusOK)
			sessionUser4.MakeRequest(t, NewRequest(t, "GET", targetLink), http.StatusNotFound)

			// Empty repository
			targetLink = path.Join("org41", "repo61", operation, "master", "README.md")
			sessionUser1.MakeRequest(t, NewRequest(t, "GET", targetLink), http.StatusNotFound)
		})
	}
}
