// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/queue"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/services/automerge"
	"code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func testPullMerge(t *testing.T, session *TestSession, user, repo, pullnum string, mergeStyle repo_model.MergeStyle, deleteBranch bool) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link := path.Join(user, repo, "pulls", pullnum, "merge")

	options := map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"do":    string(mergeStyle),
	}

	if deleteBranch {
		options["delete_branch_after_merge"] = "on"
	}

	req = NewRequestWithValues(t, "POST", link, options)
	resp = session.MakeRequest(t, req, http.StatusOK)

	respJSON := struct {
		Redirect string
	}{}
	DecodeJSON(t, resp, &respJSON)

	assert.EqualValues(t, fmt.Sprintf("/%s/%s/pulls/%s", user, repo, pullnum), respJSON.Redirect)

	return resp
}

func testPullCleanUp(t *testing.T, session *TestSession, user, repo, pullnum string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".timeline-item .delete-button").Attr("data-url")
	assert.True(t, exists, "The template has changed, can not find delete button url")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestPullMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(db.DefaultContext, 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleMerge, false)

		hookTasks, err = webhook.HookTasks(db.DefaultContext, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullRebase(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(db.DefaultContext, 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleRebase, false)

		hookTasks, err = webhook.HookTasks(db.DefaultContext, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullRebaseMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(db.DefaultContext, 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleRebaseMerge, false)

		hookTasks, err = webhook.HookTasks(db.DefaultContext, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullSquash(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		hookTasks, err := webhook.HookTasks(db.DefaultContext, 1, 1) // Retrieve previous hook number
		assert.NoError(t, err)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited!)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleSquash, false)

		hookTasks, err = webhook.HookTasks(db.DefaultContext, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, hookTasks, hookTasksLenBefore+1)
	})
}

func TestPullCleanUpAfterMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited - TestPullCleanUpAfterMerge)\n")

		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "feature/test", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleMerge, false)

		// Check PR branch deletion
		resp = testPullCleanUp(t, session, elem[1], elem[2], elem[4])
		respJSON := struct {
			Redirect string
		}{}
		DecodeJSON(t, resp, &respJSON)

		assert.NotEmpty(t, respJSON.Redirect, "Redirected URL is not found")

		elem = strings.Split(respJSON.Redirect, "/")
		assert.EqualValues(t, "pulls", elem[3])

		// Check branch deletion result
		req := NewRequest(t, "GET", respJSON.Redirect)
		resp = session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		resultMsg := htmlDoc.doc.Find(".ui.message>p").Text()

		assert.EqualValues(t, "Branch \"user1/repo1:feature/test\" has been deleted.", resultMsg)
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

		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo1)
		assert.NoError(t, err)

		err = pull.Merge(context.Background(), pr, user1, gitRepo, repo_model.MergeStyleMerge, "", "CONFLICT", false)
		assert.Error(t, err, "Merge should return an error due to conflict")
		assert.True(t, models.IsErrMergeConflicts(err), "Merge error is not a conflict error")

		err = pull.Merge(context.Background(), pr, user1, gitRepo, repo_model.MergeStyleRebase, "", "CONFLICT", false)
		assert.Error(t, err, "Merge should return an error due to conflict")
		assert.True(t, models.IsErrRebaseConflicts(err), "Merge error is not a conflict error")
		gitRepo.Close()
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

		err := git.NewCommand(git.DefaultContext, "read-tree", "--empty").Run(&git.RunOpts{Dir: path})
		assert.NoError(t, err)

		stdin := bytes.NewBufferString("Unrelated File")
		var stdout strings.Builder
		err = git.NewCommand(git.DefaultContext, "hash-object", "-w", "--stdin").Run(&git.RunOpts{
			Dir:    path,
			Stdin:  stdin,
			Stdout: &stdout,
		})

		assert.NoError(t, err)
		sha := strings.TrimSpace(stdout.String())

		_, _, err = git.NewCommand(git.DefaultContext, "update-index", "--add", "--replace", "--cacheinfo").AddDynamicArguments("100644", sha, "somewher-over-the-rainbow").RunStdString(&git.RunOpts{Dir: path})
		assert.NoError(t, err)

		treeSha, _, err := git.NewCommand(git.DefaultContext, "write-tree").RunStdString(&git.RunOpts{Dir: path})
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
		err = git.NewCommand(git.DefaultContext, "commit-tree").AddDynamicArguments(treeSha).
			Run(&git.RunOpts{
				Env:    env,
				Dir:    path,
				Stdin:  messageBytes,
				Stdout: &stdout,
			})
		assert.NoError(t, err)
		commitSha := strings.TrimSpace(stdout.String())

		_, _, err = git.NewCommand(git.DefaultContext, "branch", "unrelated").AddDynamicArguments(commitSha).RunStdString(&git.RunOpts{Dir: path})
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
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo1)
		assert.NoError(t, err)
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "unrelated",
			BaseBranch: "base",
		})

		err = pull.Merge(context.Background(), pr, user1, gitRepo, repo_model.MergeStyleMerge, "", "UNRELATED", false)
		assert.Error(t, err, "Merge should return an error due to unrelated")
		assert.True(t, models.IsErrMergeUnrelatedHistories(err), "Merge error is not a unrelated histories error")
		gitRepo.Close()
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

		gitRepo, err := git.OpenRepository(git.DefaultContext, repo_model.RepoPath(user1.Name, repo1.Name))
		assert.NoError(t, err)

		err = pull.Merge(context.Background(), pr, user1, gitRepo, repo_model.MergeStyleFastForwardOnly, "", "FAST-FORWARD-ONLY", false)

		assert.NoError(t, err)

		gitRepo.Close()
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

		gitRepo, err := git.OpenRepository(git.DefaultContext, repo_model.RepoPath(user1.Name, repo1.Name))
		assert.NoError(t, err)

		err = pull.Merge(context.Background(), pr, user1, gitRepo, repo_model.MergeStyleFastForwardOnly, "", "DIVERGING", false)

		assert.Error(t, err, "Merge should return an error due to being for a diverging branch")
		assert.True(t, models.IsErrMergeDivergingFastForwardOnly(err), "Merge error is not a diverging fast-forward-only error")

		gitRepo.Close()
	})
}

func TestConflictChecking(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// Create new clean repo to test conflict checking.
		baseRepo, err := repo_service.CreateRepository(db.DefaultContext, user, user, repo_service.CreateRepoOptions{
			Name:          "conflict-checking",
			Description:   "Tempo repo",
			AutoInit:      true,
			Readme:        "Default",
			DefaultBranch: "main",
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, baseRepo)

		// create a commit on new branch.
		_, err = files_service.ChangeRepoFiles(git.DefaultContext, baseRepo, user, &files_service.ChangeRepoFilesOptions{
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
		_, err = files_service.ChangeRepoFiles(git.DefaultContext, baseRepo, user, &files_service.ChangeRepoFilesOptions{
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
		err = pull.NewPullRequest(git.DefaultContext, baseRepo, pullIssue, nil, nil, pullRequest, nil)
		assert.NoError(t, err)

		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "PR with conflict!"})
		assert.NoError(t, issue.LoadPullRequest(db.DefaultContext))
		conflictingPR := issue.PullRequest

		// Ensure conflictedFiles is populated.
		assert.Len(t, conflictingPR.ConflictedFiles, 1)
		// Check if status is correct.
		assert.Equal(t, issues_model.PullRequestStatusConflict, conflictingPR.Status)
		// Ensure that mergeable returns false
		assert.False(t, conflictingPR.Mergeable(db.DefaultContext))
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
		assert.EqualValues(t, "pulls", elemBasePR[3])

		respChildPR := testPullCreate(t, session, "user1", "repo1", false, "base-pr", "child-pr", "Child Pull Request")
		elemChildPR := strings.Split(test.RedirectURL(respChildPR), "/")
		assert.EqualValues(t, "pulls", elemChildPR[3])

		testPullMerge(t, session, elemBasePR[1], elemBasePR[2], elemBasePR[4], repo_model.MergeStyleMerge, true)

		// Check child PR
		req := NewRequest(t, "GET", test.RedirectURL(respChildPR))
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		targetBranch := htmlDoc.doc.Find("#branch_target>a").Text()
		prStatus := strings.TrimSpace(htmlDoc.doc.Find(".issue-title-meta>.issue-state-label").Text())

		assert.EqualValues(t, "master", targetBranch)
		assert.EqualValues(t, "Open", prStatus)
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
		assert.EqualValues(t, "pulls", elemBasePR[3])

		respChildPR := testPullCreate(t, session, "user1", "repo1", true, "base-pr", "child-pr", "Child Pull Request")
		elemChildPR := strings.Split(test.RedirectURL(respChildPR), "/")
		assert.EqualValues(t, "pulls", elemChildPR[3])

		testPullMerge(t, session, elemBasePR[1], elemBasePR[2], elemBasePR[4], repo_model.MergeStyleMerge, true)

		// Check child PR
		req := NewRequest(t, "GET", test.RedirectURL(respChildPR))
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		targetBranch := htmlDoc.doc.Find("#branch_target>a").Text()
		prStatus := strings.TrimSpace(htmlDoc.doc.Find(".issue-title-meta>.issue-state-label").Text())

		assert.EqualValues(t, "base-pr", targetBranch)
		assert.EqualValues(t, "Closed", prStatus)
	})
}

func TestPullMergeIndexerNotifier(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		createPullResp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "Indexer notifier test pull")

		assert.NoError(t, queue.GetManager().FlushAll(context.Background(), 0))
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
		assert.Len(t, apiIssuesBefore, 0)

		// merge the pull request
		elem := strings.Split(test.RedirectURL(createPullResp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleMerge, false)

		// check if the issue is closed
		issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
			ID: issue.ID,
		})
		assert.True(t, issue.IsClosed)

		assert.NoError(t, queue.GetManager().FlushAll(context.Background(), 0))
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

func testResetRepo(t *testing.T, repoPath, branch, commitID string) {
	f, err := os.OpenFile(filepath.Join(repoPath, "refs", "heads", branch), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	assert.NoError(t, err)
	_, err = f.WriteString(commitID + "\n")
	assert.NoError(t, err)
	f.Close()

	repo, err := git.OpenRepository(context.Background(), repoPath)
	assert.NoError(t, err)
	defer repo.Close()
	id, err := repo.GetBranchCommitID(branch)
	assert.NoError(t, err)
	assert.EqualValues(t, commitID, id)
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
		csrf := GetCSRF(t, session, "/user2/repo1/settings/branches")
		// Change master branch to protected
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":                 csrf,
			"rule_name":             "master",
			"enable_push":           "true",
			"enable_status_check":   "true",
			"status_check_contexts": "gitea/actions",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// first time insert automerge record, return true
		scheduled, err := automerge.ScheduleAutoMerge(db.DefaultContext, user1, pr, repo_model.MergeStyleMerge, "auto merge test")
		assert.NoError(t, err)
		assert.True(t, scheduled)

		// second time insert automerge record, return false because it does exist
		scheduled, err = automerge.ScheduleAutoMerge(db.DefaultContext, user1, pr, repo_model.MergeStyleMerge, "auto merge test")
		assert.Error(t, err)
		assert.False(t, scheduled)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// update commit status to success, then it should be merged automatically
		baseGitRepo, err := gitrepo.OpenRepository(db.DefaultContext, baseRepo)
		assert.NoError(t, err)
		sha, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
		assert.NoError(t, err)
		masterCommitID, err := baseGitRepo.GetBranchCommitID("master")
		assert.NoError(t, err)

		branches, _, err := baseGitRepo.GetBranchNames(0, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"sub-home-md-img-check", "home-md-img-check", "pr-to-update", "branch2", "DefaultBranch", "develop", "feature/1", "master"}, branches)
		baseGitRepo.Close()
		defer func() {
			testResetRepo(t, baseRepo.RepoPath(), "master", masterCommitID)
		}()

		err = commitstatus_service.CreateCommitStatus(db.DefaultContext, baseRepo, user1, sha, &git_model.CommitStatus{
			State:     api.CommitStatusSuccess,
			TargetURL: "https://gitea.com",
			Context:   "gitea/actions",
		})
		assert.NoError(t, err)

		time.Sleep(2 * time.Second)

		// realod pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.True(t, pr.HasMerged)
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
		csrf := GetCSRF(t, session, "/user2/repo1/settings/branches")
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
		scheduled, err := automerge.ScheduleAutoMerge(db.DefaultContext, user1, pr, repo_model.MergeStyleMerge, "auto merge test")
		assert.NoError(t, err)
		assert.True(t, scheduled)

		// second time insert automerge record, return false because it does exist
		scheduled, err = automerge.ScheduleAutoMerge(db.DefaultContext, user1, pr, repo_model.MergeStyleMerge, "auto merge test")
		assert.Error(t, err)
		assert.False(t, scheduled)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// update commit status to success, then it should be merged automatically
		baseGitRepo, err := gitrepo.OpenRepository(db.DefaultContext, baseRepo)
		assert.NoError(t, err)
		sha, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
		assert.NoError(t, err)
		masterCommitID, err := baseGitRepo.GetBranchCommitID("master")
		assert.NoError(t, err)
		baseGitRepo.Close()
		defer func() {
			testResetRepo(t, baseRepo.RepoPath(), "master", masterCommitID)
		}()

		err = commitstatus_service.CreateCommitStatus(db.DefaultContext, baseRepo, user1, sha, &git_model.CommitStatus{
			State:     api.CommitStatusSuccess,
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

		// realod pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.True(t, pr.HasMerged)
		assert.NotEmpty(t, pr.MergedCommitID)

		unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	})
}
