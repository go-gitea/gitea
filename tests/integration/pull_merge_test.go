// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/automerge"
	"code.gitea.io/gitea/services/automergequeue"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MergeOptions struct {
	Style        repo_model.MergeStyle
	HeadCommitID string
	DeleteBranch bool
}

func testPullMerge(t *testing.T, session *TestSession, user, repo, pullnum string, mergeOptions MergeOptions) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link := path.Join(user, repo, "pulls", pullnum, "merge")

	options := map[string]string{
		"_csrf":          htmlDoc.GetCSRF(),
		"do":             string(mergeOptions.Style),
		"head_commit_id": mergeOptions.HeadCommitID,
	}

	if mergeOptions.DeleteBranch {
		options["delete_branch_after_merge"] = "on"
	}

	req = NewRequestWithValues(t, "POST", link, options)
	resp = session.MakeRequest(t, req, http.StatusOK)

	respJSON := struct {
		Redirect string
	}{}
	DecodeJSON(t, resp, &respJSON)

	assert.Equal(t, fmt.Sprintf("/%s/%s/pulls/%s", user, repo, pullnum), respJSON.Redirect)

	pullnumInt, err := strconv.ParseInt(pullnum, 10, 64)
	assert.NoError(t, err)
	repository, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), user, repo)
	assert.NoError(t, err)
	pull, err := issues_model.GetPullRequestByIndex(t.Context(), repository.ID, pullnumInt)
	assert.NoError(t, err)
	assert.True(t, pull.HasMerged)

	return resp
}

func testPullCleanUp(t *testing.T, session *TestSession, user, repo, pullnum string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".timeline-item .delete-branch-after-merge").Attr("data-url")
	assert.True(t, exists, "The template has changed, can not find delete button url")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestPullMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(t.Context(), 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 4, repo.NumOpenPulls)

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleMerge,
			DeleteBranch: false,
		})

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		hookTasks, err = webhook.HookTasks(t.Context(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullRebase(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(t.Context(), 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 4, repo.NumOpenPulls)

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleRebase,
			DeleteBranch: false,
		})

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		hookTasks, err = webhook.HookTasks(t.Context(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullRebaseMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(t.Context(), 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 4, repo.NumOpenPulls)

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleRebaseMerge,
			DeleteBranch: false,
		})

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		hookTasks, err = webhook.HookTasks(t.Context(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullSquash(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(t.Context(), 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited!)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleSquash,
			DeleteBranch: false,
		})

		hookTasks, err = webhook.HookTasks(t.Context(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullSquashWithHeadCommitID(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(t.Context(), 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited!)\n")

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: "repo1"})
		headBranch, err := git_model.GetBranch(t.Context(), repo1.ID, "master")
		assert.NoError(t, err)
		assert.NotNil(t, headBranch)

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 4, repo.NumOpenPulls)

		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleSquash,
			DeleteBranch: false,
			HeadCommitID: headBranch.CommitID,
		})
		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		hookTasks, err = webhook.HookTasks(t.Context(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullCleanUpAfterMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited - TestPullCleanUpAfterMerge)\n")

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		assert.Equal(t, 3, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "feature/test", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 4, repo.NumOpenPulls)

		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleMerge,
			DeleteBranch: false,
		})

		repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
		assert.Equal(t, 4, repo.NumPulls)
		assert.Equal(t, 3, repo.NumOpenPulls)

		// Check PR branch deletion
		resp = testPullCleanUp(t, session, elem[1], elem[2], elem[4])
		respJSON := struct {
			Redirect string
		}{}
		DecodeJSON(t, resp, &respJSON)

		assert.NotEmpty(t, respJSON.Redirect, "Redirected URL is not found")

		elem = strings.Split(respJSON.Redirect, "/")
		assert.Equal(t, "pulls", elem[3])

		// Check branch deletion result
		req := NewRequest(t, "GET", respJSON.Redirect)
		resp = session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		resultMsg := htmlDoc.doc.Find(".ui.message>p").Text()

		assert.Equal(t, "Branch \"user1/repo1:feature/test\" has been deleted.", resultMsg)
	})
}

func TestCantMergeWorkInProgress(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "[wip] This is a pull title")

		req := NewRequest(t, "GET", test.RedirectURL(resp))
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		text := strings.TrimSpace(htmlDoc.doc.Find(".merge-section > .item").Last().Text())
		assert.NotEmpty(t, text, "Can't find WIP text")

		assert.Contains(t, text, translation.NewLocale("en-US").TrString("repo.pulls.cannot_merge_work_in_progress"), "Unable to find WIP text")
		assert.Contains(t, text, "[wip]", "Unable to find WIP text")
	})
}

func TestCantMergeConflict(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "conflict", "README.md", "Hello, World (Edited Once)\n")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "base", "README.md", "Hello, World (Edited Twice)\n")

		// Use API to create a conflicting pr
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", "user1", "repo1"), &api.CreatePullRequestOption{
			Head:  "conflict",
			Base:  "base",
			Title: "create a conflicting pr",
		}).AddTokenAuth(token)
		session.MakeRequest(t, req, http.StatusCreated)

		// Now this PR will be marked conflict - or at least a race will do - so drop down to pure code at this point...
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: "user1",
		})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		})

		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "conflict",
			BaseBranch: "base",
		})

		err := pull_service.Merge(t.Context(), pr, user1, repo_model.MergeStyleMerge, "", "CONFLICT", false)
		assert.Error(t, err, "Merge should return an error due to conflict")
		assert.True(t, pull_service.IsErrMergeConflicts(err), "Merge error is not a conflict error")

		err = pull_service.Merge(t.Context(), pr, user1, repo_model.MergeStyleRebase, "", "CONFLICT", false)
		assert.Error(t, err, "Merge should return an error due to conflict")
		assert.True(t, pull_service.IsErrRebaseConflicts(err), "Merge error is not a conflict error")
	})
}

func TestCantMergeUnrelated(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "base", "README.md", "Hello, World (Edited Twice)\n")

		// Now we want to create a commit on a branch that is totally unrelated to our current head
		// Drop down to pure code at this point
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: "user1",
		})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		})
		path := repo_model.RepoPath(user1.Name, repo1.Name)

		err := gitcmd.NewCommand("read-tree", "--empty").WithDir(path).Run(t.Context())
		assert.NoError(t, err)

		stdin := strings.NewReader("Unrelated File")
		var stdout strings.Builder
		err = gitcmd.NewCommand("hash-object", "-w", "--stdin").
			WithDir(path).
			WithStdin(stdin).
			WithStdout(&stdout).
			Run(t.Context())

		assert.NoError(t, err)
		sha := strings.TrimSpace(stdout.String())

		_, _, err = gitcmd.NewCommand("update-index", "--add", "--replace", "--cacheinfo").
			AddDynamicArguments("100644", sha, "somewher-over-the-rainbow").
			WithDir(path).
			RunStdString(t.Context())
		assert.NoError(t, err)

		treeSha, _, err := gitcmd.NewCommand("write-tree").WithDir(path).RunStdString(t.Context())
		assert.NoError(t, err)
		treeSha = strings.TrimSpace(treeSha)

		commitTimeStr := time.Now().Format(time.RFC3339)
		doerSig := user1.NewGitSig()
		env := append(os.Environ(),
			"GIT_AUTHOR_NAME="+doerSig.Name,
			"GIT_AUTHOR_EMAIL="+doerSig.Email,
			"GIT_AUTHOR_DATE="+commitTimeStr,
			"GIT_COMMITTER_NAME="+doerSig.Name,
			"GIT_COMMITTER_EMAIL="+doerSig.Email,
			"GIT_COMMITTER_DATE="+commitTimeStr,
		)

		messageBytes := new(bytes.Buffer)
		_, _ = messageBytes.WriteString("Unrelated")
		_, _ = messageBytes.WriteString("\n")

		stdout.Reset()
		err = gitcmd.NewCommand("commit-tree").AddDynamicArguments(treeSha).
			WithEnv(env).
			WithDir(path).
			WithStdin(messageBytes).
			WithStdout(&stdout).
			Run(t.Context())
		assert.NoError(t, err)
		commitSha := strings.TrimSpace(stdout.String())

		_, _, err = gitcmd.NewCommand("branch", "unrelated").
			AddDynamicArguments(commitSha).
			WithDir(path).
			RunStdString(t.Context())
		assert.NoError(t, err)

		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "conflict", "README.md", "Hello, World (Edited Once)\n")

		// Use API to create a conflicting pr
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", "user1", "repo1"), &api.CreatePullRequestOption{
			Head:  "unrelated",
			Base:  "base",
			Title: "create an unrelated pr",
		}).AddTokenAuth(token)
		session.MakeRequest(t, req, http.StatusCreated)

		// Now this PR could be marked conflict - or at least a race may occur - so drop down to pure code at this point...
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "unrelated",
			BaseBranch: "base",
		})

		err = pull_service.Merge(t.Context(), pr, user1, repo_model.MergeStyleMerge, "", "UNRELATED", false)
		assert.Error(t, err, "Merge should return an error due to unrelated")
		assert.True(t, pull_service.IsErrMergeUnrelatedHistories(err), "Merge error is not a unrelated histories error")
	})
}

func TestFastForwardOnlyMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "update", "README.md", "Hello, World 2\n")

		// Use API to create a pr from update to master
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", "user1", "repo1"), &api.CreatePullRequestOption{
			Head:  "update",
			Base:  "master",
			Title: "create a pr that can be fast-forward-only merged",
		}).AddTokenAuth(token)
		session.MakeRequest(t, req, http.StatusCreated)

		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: "user1",
		})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		})

		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "update",
			BaseBranch: "master",
		})

		err := pull_service.Merge(t.Context(), pr, user1, repo_model.MergeStyleFastForwardOnly, "", "FAST-FORWARD-ONLY", false)
		assert.NoError(t, err)
	})
}

func TestCantFastForwardOnlyMergeDiverging(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "diverging", "README.md", "Hello, World diverged\n")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World 2\n")

		// Use API to create a pr from diverging to update
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", "user1", "repo1"), &api.CreatePullRequestOption{
			Head:  "diverging",
			Base:  "master",
			Title: "create a pr from a diverging branch",
		}).AddTokenAuth(token)
		session.MakeRequest(t, req, http.StatusCreated)

		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: "user1",
		})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		})

		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "diverging",
			BaseBranch: "master",
		})

		err := pull_service.Merge(t.Context(), pr, user1, repo_model.MergeStyleFastForwardOnly, "", "DIVERGING", false)
		assert.Error(t, err, "Merge should return an error due to being for a diverging branch")
		assert.True(t, pull_service.IsErrMergeDivergingFastForwardOnly(err), "Merge error is not a diverging fast-forward-only error")
	})
}

func TestConflictChecking(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// Create new clean repo to test conflict checking.
		baseRepo, err := repo_service.CreateRepository(t.Context(), user, user, repo_service.CreateRepoOptions{
			Name:          "conflict-checking",
			Description:   "Tempo repo",
			AutoInit:      true,
			Readme:        "Default",
			DefaultBranch: "main",
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, baseRepo)

		// create a commit on new branch.
		_, err = files_service.ChangeRepoFiles(t.Context(), baseRepo, user, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "important_file",
					ContentReader: strings.NewReader("Just a non-important file"),
				},
			},
			Message:   "Add a important file",
			OldBranch: "main",
			NewBranch: "important-secrets",
		})
		assert.NoError(t, err)

		// create a commit on main branch.
		_, err = files_service.ChangeRepoFiles(t.Context(), baseRepo, user, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "important_file",
					ContentReader: strings.NewReader("Not the same content :P"),
				},
			},
			Message:   "Add a important file",
			OldBranch: "main",
			NewBranch: "main",
		})
		assert.NoError(t, err)

		// create Pull to merge the important-secrets branch into main branch.
		pullIssue := &issues_model.Issue{
			RepoID:   baseRepo.ID,
			Title:    "PR with conflict!",
			PosterID: user.ID,
			Poster:   user,
			IsPull:   true,
		}

		pullRequest := &issues_model.PullRequest{
			HeadRepoID: baseRepo.ID,
			BaseRepoID: baseRepo.ID,
			HeadBranch: "important-secrets",
			BaseBranch: "main",
			HeadRepo:   baseRepo,
			BaseRepo:   baseRepo,
			Type:       issues_model.PullRequestGitea,
		}
		prOpts := &pull_service.NewPullRequestOptions{Repo: baseRepo, Issue: pullIssue, PullRequest: pullRequest}
		err = pull_service.NewPullRequest(t.Context(), prOpts)
		assert.NoError(t, err)

		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "PR with conflict!"})
		assert.NoError(t, issue.LoadPullRequest(t.Context()))
		conflictingPR := issue.PullRequest

		// Ensure conflictedFiles is populated.
		assert.Len(t, conflictingPR.ConflictedFiles, 1)
		// Check if status is correct.
		assert.Equal(t, issues_model.PullRequestStatusConflict, conflictingPR.Status)
		// Ensure that mergeable returns false
		assert.False(t, conflictingPR.Mergeable(t.Context()))
	})
}

func TestPullRetargetChildOnBranchDelete(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testEditFileToNewBranch(t, session, "user2", "repo1", "master", "base-pr", "README.md", "Hello, World\n(Edited - TestPullRetargetOnCleanup - base PR)\n")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "base-pr", "child-pr", "README.md", "Hello, World\n(Edited - TestPullRetargetOnCleanup - base PR)\n(Edited - TestPullRetargetOnCleanup - child PR)")

		respBasePR := testPullCreate(t, session, "user2", "repo1", true, "master", "base-pr", "Base Pull Request")
		elemBasePR := strings.Split(test.RedirectURL(respBasePR), "/")
		assert.Equal(t, "pulls", elemBasePR[3])

		respChildPR := testPullCreate(t, session, "user1", "repo1", false, "base-pr", "child-pr", "Child Pull Request")
		elemChildPR := strings.Split(test.RedirectURL(respChildPR), "/")
		assert.Equal(t, "pulls", elemChildPR[3])

		testPullMerge(t, session, elemBasePR[1], elemBasePR[2], elemBasePR[4], MergeOptions{
			Style:        repo_model.MergeStyleMerge,
			DeleteBranch: true,
		})

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		branchBasePR := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "base-pr"})
		assert.True(t, branchBasePR.IsDeleted)

		// Check child PR
		req := NewRequest(t, "GET", test.RedirectURL(respChildPR))
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		targetBranch := htmlDoc.doc.Find("#branch_target>a").Text()
		prStatus := strings.TrimSpace(htmlDoc.doc.Find(".issue-title-meta>.issue-state-label").Text())

		assert.Equal(t, "master", targetBranch)
		assert.Equal(t, "Open", prStatus)
	})
}

func TestPullDontRetargetChildOnWrongRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "base-pr", "README.md", "Hello, World\n(Edited - TestPullDontRetargetChildOnWrongRepo - base PR)\n")
		testEditFileToNewBranch(t, session, "user1", "repo1", "base-pr", "child-pr", "README.md", "Hello, World\n(Edited - TestPullDontRetargetChildOnWrongRepo - base PR)\n(Edited - TestPullDontRetargetChildOnWrongRepo - child PR)")

		respBasePR := testPullCreate(t, session, "user1", "repo1", false, "master", "base-pr", "Base Pull Request")
		elemBasePR := strings.Split(test.RedirectURL(respBasePR), "/")
		assert.Equal(t, "pulls", elemBasePR[3])

		respChildPR := testPullCreate(t, session, "user1", "repo1", true, "base-pr", "child-pr", "Child Pull Request")
		elemChildPR := strings.Split(test.RedirectURL(respChildPR), "/")
		assert.Equal(t, "pulls", elemChildPR[3])

		defer test.MockVariableValue(&setting.Repository.PullRequest.RetargetChildrenOnMerge, false)()

		testPullMerge(t, session, elemBasePR[1], elemBasePR[2], elemBasePR[4], MergeOptions{
			Style:        repo_model.MergeStyleMerge,
			DeleteBranch: true,
		})

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: "repo1"})
		branchBasePR := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "base-pr"})
		assert.True(t, branchBasePR.IsDeleted)

		// Check child PR
		req := NewRequest(t, "GET", test.RedirectURL(respChildPR))
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		// the branch has been deleted, so there is no a html tag instead of span
		targetBranch := htmlDoc.doc.Find("#branch_target>span").Text()
		prStatus := strings.TrimSpace(htmlDoc.doc.Find(".issue-title-meta>.issue-state-label").Text())

		assert.Equal(t, "base-pr", targetBranch)
		assert.Equal(t, "Closed", prStatus)
	})
}

func TestPullRequestMergedWithNoPermissionDeleteBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user4")
		testRepoFork(t, session, "user2", "repo1", "user4", "repo1", "")
		testEditFileToNewBranch(t, session, "user4", "repo1", "master", "base-pr", "README.md", "Hello, World\n(Edited - TestPullDontRetargetChildOnWrongRepo - base PR)\n")

		respBasePR := testPullCreate(t, session, "user4", "repo1", false, "master", "base-pr", "Base Pull Request")
		elemBasePR := strings.Split(test.RedirectURL(respBasePR), "/")
		assert.Equal(t, "pulls", elemBasePR[3])

		// user2 has no permission to delete branch of repo user1/repo1
		session2 := loginUser(t, "user2")
		testPullMerge(t, session2, elemBasePR[1], elemBasePR[2], elemBasePR[4], MergeOptions{
			Style:        repo_model.MergeStyleMerge,
			DeleteBranch: true,
		})

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user4", Name: "repo1"})
		branchBasePR := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo1.ID, Name: "base-pr"})
		// branch has not been deleted
		assert.False(t, branchBasePR.IsDeleted)
	})
}

func TestPullMergeIndexerNotifier(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		createPullResp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "Indexer notifier test pull")

		assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 0))
		time.Sleep(time.Second)

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{
			OwnerName: "user2",
			Name:      "repo1",
		})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
			RepoID:   repo1.ID,
			Title:    "Indexer notifier test pull",
			IsPull:   true,
			IsClosed: false,
		})

		// build the request for searching issues
		link, _ := url.Parse("/api/v1/repos/issues/search")
		query := url.Values{}
		query.Add("state", "closed")
		query.Add("type", "pulls")
		query.Add("q", "notifier")
		link.RawQuery = query.Encode()

		// search issues
		searchIssuesResp := session.MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
		var apiIssuesBefore []*api.Issue
		DecodeJSON(t, searchIssuesResp, &apiIssuesBefore)
		assert.Empty(t, apiIssuesBefore)

		// merge the pull request
		elem := strings.Split(test.RedirectURL(createPullResp), "/")
		assert.Equal(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleMerge,
			DeleteBranch: false,
		})

		// check if the issue is closed
		issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
			ID: issue.ID,
		})
		assert.True(t, issue.IsClosed)

		assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 0))
		time.Sleep(time.Second)

		// search issues again
		searchIssuesResp = session.MakeRequest(t, NewRequest(t, "GET", link.String()), http.StatusOK)
		var apiIssuesAfter []*api.Issue
		DecodeJSON(t, searchIssuesResp, &apiIssuesAfter)
		if assert.Len(t, apiIssuesAfter, 1) {
			assert.Equal(t, issue.ID, apiIssuesAfter[0].ID)
		}
	})
}

func testResetRepo(t *testing.T, repo *repo_model.Repository, branch, commitID string) {
	assert.NoError(t, gitrepo.UpdateRef(t.Context(), repo, git.BranchPrefix+branch, commitID))

	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	assert.NoError(t, err)
	defer gitRepo.Close()
	id, err := gitRepo.GetBranchCommitID(branch)
	assert.NoError(t, err)
	assert.Equal(t, commitID, id)
}

func TestPullAutoMergeAfterCommitStatusSucceed(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		forkedName := "repo1-1"
		testRepoFork(t, session, "user2", "repo1", "user1", forkedName, "")
		defer func() {
			testDeleteRepository(t, session, "user1", forkedName)
		}()
		testEditFile(t, session, "user1", forkedName, "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", forkedName, false, "master", "master", "Indexer notifier test pull")

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		forkedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: forkedName})
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: baseRepo.ID,
			BaseBranch: "master",
			HeadRepoID: forkedRepo.ID,
			HeadBranch: "master",
		})

		// add protected branch for commit status
		csrf := GetUserCSRFToken(t, session)
		// Change the "master" branch to "protected"
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":                 csrf,
			"rule_name":             "master",
			"enable_push":           "true",
			"enable_status_check":   "true",
			"status_check_contexts": "gitea/actions",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		oldAutoMergeAddToQueue := automergequeue.AddToQueue
		addToQueueShaChan := make(chan string, 1)
		automergequeue.AddToQueue = func(pr *issues_model.PullRequest, sha string) {
			addToQueueShaChan <- sha
		}
		// first time insert automerge record, return true
		scheduled, err := automerge.ScheduleAutoMerge(t.Context(), user1, pr, repo_model.MergeStyleMerge, "auto merge test", false)
		assert.NoError(t, err)
		assert.True(t, scheduled)
		// and the pr should be added to automergequeue, in case it is already "mergeable"
		select {
		case <-addToQueueShaChan:
		case <-time.After(time.Second):
			assert.FailNow(t, "Timeout: nothing was added to automergequeue")
		}
		automergequeue.AddToQueue = oldAutoMergeAddToQueue

		// second time insert automerge record, return false because it does exist
		scheduled, err = automerge.ScheduleAutoMerge(t.Context(), user1, pr, repo_model.MergeStyleMerge, "auto merge test", false)
		assert.Error(t, err)
		assert.False(t, scheduled)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// update commit status to success, then it should be merged automatically
		baseGitRepo, err := gitrepo.OpenRepository(t.Context(), baseRepo)
		assert.NoError(t, err)
		sha, err := baseGitRepo.GetRefCommitID(pr.GetGitHeadRefName())
		assert.NoError(t, err)
		masterCommitID, err := baseGitRepo.GetBranchCommitID("master")
		assert.NoError(t, err)

		branches, _, err := baseGitRepo.GetBranchNames(0, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"sub-home-md-img-check", "home-md-img-check", "pr-to-update", "branch2", "DefaultBranch", "develop", "feature/1", "master"}, branches)
		baseGitRepo.Close()
		defer func() {
			testResetRepo(t, baseRepo, "master", masterCommitID)
		}()

		err = commitstatus_service.CreateCommitStatus(t.Context(), baseRepo, user1, sha, &git_model.CommitStatus{
			State:     commitstatus.CommitStatusSuccess,
			TargetURL: "https://gitea.com",
			Context:   "gitea/actions",
		})
		assert.NoError(t, err)

		assert.Eventually(t, func() bool {
			pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
			return pr.HasMerged
		}, 2*time.Second, 100*time.Millisecond)
		assert.NotEmpty(t, pr.MergedCommitID)
		unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	})
}

func TestPullAutoMergeAfterCommitStatusSucceedAndApproval(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		forkedName := "repo1-2"
		testRepoFork(t, session, "user2", "repo1", "user1", forkedName, "")
		defer func() {
			testDeleteRepository(t, session, "user1", forkedName)
		}()
		testEditFile(t, session, "user1", forkedName, "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", forkedName, false, "master", "master", "Indexer notifier test pull")

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		forkedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: forkedName})
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: baseRepo.ID,
			BaseBranch: "master",
			HeadRepoID: forkedRepo.ID,
			HeadBranch: "master",
		})

		// add protected branch for commit status
		csrf := GetUserCSRFToken(t, session)
		// Change master branch to protected
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":                 csrf,
			"rule_name":             "master",
			"enable_push":           "true",
			"enable_status_check":   "true",
			"status_check_contexts": "gitea/actions",
			"required_approvals":    "1",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// first time insert automerge record, return true
		scheduled, err := automerge.ScheduleAutoMerge(t.Context(), user1, pr, repo_model.MergeStyleMerge, "auto merge test", false)
		assert.NoError(t, err)
		assert.True(t, scheduled)

		// second time insert automerge record, return false because it does exist
		scheduled, err = automerge.ScheduleAutoMerge(t.Context(), user1, pr, repo_model.MergeStyleMerge, "auto merge test", false)
		assert.Error(t, err)
		assert.False(t, scheduled)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// update commit status to success, then it should be merged automatically
		baseGitRepo, err := gitrepo.OpenRepository(t.Context(), baseRepo)
		assert.NoError(t, err)
		sha, err := baseGitRepo.GetRefCommitID(pr.GetGitHeadRefName())
		assert.NoError(t, err)
		masterCommitID, err := baseGitRepo.GetBranchCommitID("master")
		assert.NoError(t, err)
		baseGitRepo.Close()
		defer func() {
			testResetRepo(t, baseRepo, "master", masterCommitID)
		}()

		err = commitstatus_service.CreateCommitStatus(t.Context(), baseRepo, user1, sha, &git_model.CommitStatus{
			State:     commitstatus.CommitStatusSuccess,
			TargetURL: "https://gitea.com",
			Context:   "gitea/actions",
		})
		assert.NoError(t, err)

		time.Sleep(2 * time.Second)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// approve the PR from non-author
		approveSession := loginUser(t, "user2")
		req = NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/pulls/%d", pr.Index))
		resp := approveSession.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		testSubmitReview(t, approveSession, htmlDoc.GetCSRF(), "user2", "repo1", strconv.Itoa(int(pr.Index)), sha, "approve", http.StatusOK)

		time.Sleep(2 * time.Second)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.True(t, pr.HasMerged)
		assert.NotEmpty(t, pr.MergedCommitID)

		unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	})
}

func TestPullAutoMergeAfterCommitStatusSucceedAndApprovalForAgitFlow(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// create a pull request
		baseAPITestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		dstPath := t.TempDir()

		u.Path = baseAPITestContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		err := os.WriteFile(path.Join(dstPath, "test_file"), []byte("## test content"), 0o666)
		assert.NoError(t, err)

		err = git.AddChanges(t.Context(), dstPath, true)
		assert.NoError(t, err)

		err = git.CommitChanges(t.Context(), dstPath, git.CommitChangesOptions{
			Committer: &git.Signature{
				Email: "user2@example.com",
				Name:  "user2",
				When:  time.Now(),
			},
			Author: &git.Signature{
				Email: "user2@example.com",
				Name:  "user2",
				When:  time.Now(),
			},
			Message: "Testing commit 1",
		})
		assert.NoError(t, err)

		stderrBuf := &bytes.Buffer{}

		err = gitcmd.NewCommand("push", "origin", "HEAD:refs/for/master", "-o").
			AddDynamicArguments(`topic=test/head2`).
			AddArguments("-o").
			AddDynamicArguments(`title="create a test pull request with agit"`).
			AddArguments("-o").
			AddDynamicArguments(`description="This PR is a test pull request which created with agit"`).
			WithDir(dstPath).
			WithStderr(stderrBuf).
			Run(t.Context())
		assert.NoError(t, err)

		assert.Contains(t, stderrBuf.String(), setting.AppURL+"user2/repo1/pulls/6")

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			Flow:       issues_model.PullRequestFlowAGit,
			BaseRepoID: baseRepo.ID,
			BaseBranch: "master",
			HeadRepoID: baseRepo.ID,
			HeadBranch: "user2/test/head2",
		})

		session := loginUser(t, "user1")
		// add protected branch for commit status
		csrf := GetUserCSRFToken(t, session)
		// Change master branch to protected
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":                 csrf,
			"rule_name":             "master",
			"enable_push":           "true",
			"enable_status_check":   "true",
			"status_check_contexts": "gitea/actions",
			"required_approvals":    "1",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		// first time insert automerge record, return true
		scheduled, err := automerge.ScheduleAutoMerge(t.Context(), user1, pr, repo_model.MergeStyleMerge, "auto merge test", false)
		assert.NoError(t, err)
		assert.True(t, scheduled)

		// second time insert automerge record, return false because it does exist
		scheduled, err = automerge.ScheduleAutoMerge(t.Context(), user1, pr, repo_model.MergeStyleMerge, "auto merge test", false)
		assert.Error(t, err)
		assert.False(t, scheduled)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// update commit status to success, then it should be merged automatically
		baseGitRepo, err := gitrepo.OpenRepository(t.Context(), baseRepo)
		assert.NoError(t, err)
		sha, err := baseGitRepo.GetRefCommitID(pr.GetGitHeadRefName())
		assert.NoError(t, err)
		masterCommitID, err := baseGitRepo.GetBranchCommitID("master")
		assert.NoError(t, err)
		baseGitRepo.Close()
		defer func() {
			testResetRepo(t, baseRepo, "master", masterCommitID)
		}()

		err = commitstatus_service.CreateCommitStatus(t.Context(), baseRepo, user1, sha, &git_model.CommitStatus{
			State:     commitstatus.CommitStatusSuccess,
			TargetURL: "https://gitea.com",
			Context:   "gitea/actions",
		})
		assert.NoError(t, err)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// approve the PR from non-author
		approveSession := loginUser(t, "user1")
		req = NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/pulls/%d", pr.Index))
		resp := approveSession.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		testSubmitReview(t, approveSession, htmlDoc.GetCSRF(), "user2", "repo1", strconv.Itoa(int(pr.Index)), sha, "approve", http.StatusOK)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.True(t, pr.HasMerged)
		assert.NotEmpty(t, pr.MergedCommitID)

		unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	})
}

func TestPullNonMergeForAdminWithBranchProtection(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		forkedName := "repo1-1"
		testRepoFork(t, session, "user2", "repo1", "user1", forkedName, "")
		defer testDeleteRepository(t, session, "user1", forkedName)

		testEditFile(t, session, "user1", forkedName, "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", forkedName, false, "master", "master", "Indexer notifier test pull")

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		forkedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: forkedName})
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: baseRepo.ID,
			BaseBranch: "master",
			HeadRepoID: forkedRepo.ID,
			HeadBranch: "master",
		})

		// add protected branch for commit status
		csrf := GetUserCSRFToken(t, session)
		// Change master branch to protected
		pbCreateReq := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":                      csrf,
			"rule_name":                  "master",
			"enable_push":                "true",
			"enable_status_check":        "true",
			"status_check_contexts":      "gitea/actions",
			"block_admin_merge_override": "true",
		})
		session.MakeRequest(t, pbCreateReq, http.StatusSeeOther)

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		mergeReq := NewRequestWithValues(t, "POST", "/api/v1/repos/user2/repo1/pulls/6/merge", map[string]string{
			"_csrf":                     csrf,
			"head_commit_id":            "",
			"merge_when_checks_succeed": "false",
			"force_merge":               "true",
			"do":                        "rebase",
		}).AddTokenAuth(token)

		session.MakeRequest(t, mergeReq, http.StatusMethodNotAllowed)
	})
}

func TestPullSquashMergeEmpty(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testEditFileToNewBranch(t, session, "user2", "repo1", "master", "pr-squash-empty", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user2", "repo1", false, "master", "pr-squash-empty", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])

		httpContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)
		dstPath := t.TempDir()

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		t.Run("Clone", doGitClone(dstPath, u))
		doGitCheckoutBranch(dstPath, "-b", "pr-squash-empty", "remotes/origin/pr-squash-empty")(t)
		doGitCheckoutBranch(dstPath, "master")(t)
		_, _, err := gitcmd.NewCommand("cherry-pick").AddArguments("pr-squash-empty").
			WithDir(dstPath).
			RunStdString(t.Context())
		assert.NoError(t, err)

		doGitPushTestRepository(dstPath)(t)

		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleSquash,
			DeleteBranch: false,
		})
	})
}

func TestPullSquashMessage(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)

		defer test.MockVariableValue(&setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages, true)()
		defer test.MockVariableValue(&setting.Repository.PullRequest.DefaultMergeMessageSize, 80)()

		repo, err := repo_service.CreateRepository(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:          "squash-message-test",
			Description:   "Test squash message",
			AutoInit:      true,
			Readme:        "Default",
			DefaultBranch: "main",
		})
		require.NoError(t, err)

		type commitInfo struct {
			userName      string
			commitMessage string
		}

		testCases := []struct {
			name            string
			commitInfos     []*commitInfo
			expectedMessage string
		}{
			{
				name: "Single-line messages",
				commitInfos: []*commitInfo{
					{
						userName:      user2.Name,
						commitMessage: "commit msg 1",
					},
					{
						userName:      user2.Name,
						commitMessage: "commit msg 2",
					},
				},
				expectedMessage: `* commit msg 1

* commit msg 2

`,
			},
			{
				name: "Multiple-line messages",
				commitInfos: []*commitInfo{
					{
						userName: user2.Name,
						commitMessage: `commit msg 1

Commit description.`,
					},
					{
						userName: user2.Name,
						commitMessage: `commit msg 2

- Detail 1
- Detail 2`,
					},
				},
				expectedMessage: `* commit msg 1

Commit description.

* commit msg 2

- Detail 1
- Detail 2

`,
			},
			{
				name: "Too long message",
				commitInfos: []*commitInfo{
					{
						userName:      user2.Name,
						commitMessage: `loooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong message`,
					},
				},
				expectedMessage: `* looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo...`,
			},
			{
				name: "Test Co-authored-by",
				commitInfos: []*commitInfo{
					{
						userName:      user2.Name,
						commitMessage: "commit msg 1",
					},
					{
						userName:      "user4",
						commitMessage: "commit msg 2",
					},
				},
				expectedMessage: `* commit msg 1

* commit msg 2

---------

Co-authored-by: user4 <user4@example.com>
`,
			},
		}

		for tcNum, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				branchName := "test-branch-" + strconv.Itoa(tcNum)
				for infoIdx, info := range tc.commitInfos {
					createFileOpts := createFileInBranchOptions{
						CommitMessage:  info.commitMessage,
						CommitterName:  info.userName,
						CommitterEmail: util.Iif(info.userName != "", info.userName+"@example.com", ""),
						OldBranch:      util.Iif(infoIdx == 0, "main", branchName),
						NewBranch:      branchName,
					}
					testCreateFileInBranch(t, user2, repo, createFileOpts, map[string]string{"dummy-file-" + strconv.Itoa(infoIdx): "dummy content"})
				}
				resp := testPullCreateDirectly(t, user2Session, createPullRequestOptions{
					BaseRepoOwner: user2.Name,
					BaseRepoName:  repo.Name,
					BaseBranch:    repo.DefaultBranch,
					HeadBranch:    branchName,
					Title:         "Pull for " + branchName,
				})
				elems := strings.Split(test.RedirectURL(resp), "/")
				pullIndex, err := strconv.ParseInt(elems[4], 10, 64)
				assert.NoError(t, err)
				pullRequest := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, Index: pullIndex})
				squashMergeCommitMessage := pull_service.GetSquashMergeCommitMessages(t.Context(), pullRequest)
				assert.Equal(t, tc.expectedMessage, squashMergeCommitMessage)
			})
		}
	})
}
