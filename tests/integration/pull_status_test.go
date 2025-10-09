// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestPullCreate_EmptyChangesWithDifferentCommits(t *testing.T) {
	// Merge must continue if commits SHA are different, even if content is same
	// Reason: gitflow and merging master back into develop, where is high possibility, there are no changes
	// but just commit saying "Merge branch". And this meta commit can be also tagged,
	// so we need to have this meta commit also in develop branch.
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")
		testEditFile(t, session, "user1", "repo1", "status1", "README.md", "# repo1\n\nDescription for repo1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/user1/repo1/pulls/1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		text := strings.TrimSpace(doc.doc.Find(".merge-section").Text())
		assert.Contains(t, text, "This pull request can be merged automatically.")
	})
}

func TestPullCreate_EmptyChangesWithSameCommits(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "status1", http.StatusSeeOther)
		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/user1/repo1/pulls/1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		text := strings.TrimSpace(doc.doc.Find(".merge-section").Text())
		assert.Contains(t, text, "This branch is already included in the target branch. There is nothing to merge.")
	})
}

func TestPullStatusDelayCheck(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer test.MockVariableValue(&setting.Repository.PullRequest.DelayCheckForInactiveDays, 1)()
		defer test.MockVariableValue(&pull_service.AddPullRequestToCheckQueue)()

		session := loginUser(t, "user2")

		run := func(t *testing.T, fn func(*testing.T)) (issue3 *issues_model.Issue, checkedPrID int64) {
			pull_service.AddPullRequestToCheckQueue = func(prID int64) {
				checkedPrID = prID
			}
			fn(t)
			issue3 = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 1, Index: 3})
			_ = issue3.LoadPullRequest(t.Context())
			return issue3, checkedPrID
		}

		assertReloadingInterval := func(t *testing.T, interval string) {
			req := NewRequest(t, "GET", "/user2/repo1/pulls/3")
			resp := session.MakeRequest(t, req, http.StatusOK)
			attr := "data-pull-merge-box-reloading-interval"
			if interval == "" {
				assert.NotContains(t, resp.Body.String(), attr)
			} else {
				assert.Contains(t, resp.Body.String(), fmt.Sprintf(`%s="%v"`, attr, interval))
			}
		}

		// PR issue3 is merageable at the beginning
		issue3, checkedPrID := run(t, func(t *testing.T) {})
		assert.Equal(t, issues_model.PullRequestStatusMergeable, issue3.PullRequest.Status)
		assert.Zero(t, checkedPrID)
		assertReloadingInterval(t, "") // the PR is mergeable, so no need to reload the merge box

		// setting.IsProd = false // it would cause data-race because the queue handlers might be running and reading its value
		// assertReloadingInterval(t, "1") // make sure dev mode always do merge box reloading, to make sure the UI logic won't break
		// setting.IsProd = true

		// when base branch changes, PR status should be updated, but it is inactive for long time, so no real check
		issue3, checkedPrID = run(t, func(t *testing.T) {
			testEditFile(t, session, "user2", "repo1", "master", "README.md", "new content 1")
		})
		assert.Equal(t, issues_model.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Zero(t, checkedPrID)
		assertReloadingInterval(t, "2000") // the PR status is "checking", so try to reload the merge box

		// view a PR with status=checking, it starts the real check
		issue3, checkedPrID = run(t, func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/pulls/3")
			session.MakeRequest(t, req, http.StatusOK)
		})
		assert.Equal(t, issues_model.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Equal(t, issue3.PullRequest.ID, checkedPrID)

		// when base branch changes, still so no real check
		issue3, checkedPrID = run(t, func(t *testing.T) {
			testEditFile(t, session, "user2", "repo1", "master", "README.md", "new content 2")
		})
		assert.Equal(t, issues_model.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Zero(t, checkedPrID)

		// then allow to check PRs without delay, when base branch changes, the PRs will be checked
		setting.Repository.PullRequest.DelayCheckForInactiveDays = -1
		issue3, checkedPrID = run(t, func(t *testing.T) {
			testEditFile(t, session, "user2", "repo1", "master", "README.md", "new content 3")
		})
		assert.Equal(t, issues_model.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Equal(t, issue3.PullRequest.ID, checkedPrID)
	})
}

func Test_PullRequestStatusChecking_Mergeable_MergeTree(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, true)()
		testPullRequestStatusCheckingMergeable(t)
	})
}

func Test_PullRequestStatusChecking_Mergeable_TmpRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, false)()
		testPullRequestStatusCheckingMergeable(t)
	})
}

func testPullRequestStatusCheckingMergeable(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)

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
	testCreateFile(t, session, baseRepo.OwnerName, baseRepo.Name, "main", "important-secrets", "important_file", "Just a non-important file")

	// create Pull to merge the important-secrets branch into main branch.
	resp := testPullCreateDirectly(t, session, baseRepo.OwnerName, baseRepo.Name, "main",
		baseRepo.OwnerName, baseRepo.Name, "important-secrets", "PR with no conflict")
	// check the redirected URL
	url := test.RedirectURL(resp)
	assert.Regexp(t, fmt.Sprintf("^/%s/pulls/[0-9]*$", baseRepo.FullName()), url)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "PR with no conflict"})
	assert.NoError(t, issue.LoadPullRequest(t.Context()))
	conflictingPR := issue.PullRequest

	// Ensure conflictedFiles is populated.
	assert.Empty(t, conflictingPR.ConflictedFiles)
	// Check if status is correct.
	assert.Equal(t, issues_model.PullRequestStatusMergeable, conflictingPR.Status)
	// Ensure that mergeable returns true
	assert.True(t, conflictingPR.Mergeable(t.Context()))
}

func Test_PullRequestStatusChecking_Conflicted_MergeTree(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, true)()
		testPullRequestStatusCheckingConflicted(t)
	})
}

func Test_PullRequestStatusChecking_Conflicted_TmpRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, false)()
		testPullRequestStatusCheckingConflicted(t)
	})
}

func testPullRequestStatusCheckingConflicted(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)

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
	testCreateFile(t, session, baseRepo.OwnerName, baseRepo.Name, "main", "important-secrets", "important_file", "Just a non-important file")

	// create a commit on main branch.
	testCreateFile(t, session, baseRepo.OwnerName, baseRepo.Name, "main", "main", "important_file", "Not the same content :P")

	// create Pull to merge the important-secrets branch into main branch.
	resp := testPullCreateDirectly(t, session, baseRepo.OwnerName, baseRepo.Name, "main",
		baseRepo.OwnerName, baseRepo.Name, "important-secrets", "PR with conflict!")
	// check the redirected URL
	url := test.RedirectURL(resp)
	assert.Regexp(t, fmt.Sprintf("^/%s/pulls/[0-9]*$", baseRepo.FullName()), url)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "PR with conflict!"})
	assert.NoError(t, issue.LoadPullRequest(t.Context()))
	conflictingPR := issue.PullRequest

	// Ensure conflictedFiles is populated.
	assert.Equal(t, []string{"important_file"}, conflictingPR.ConflictedFiles)
	// Check if status is correct.
	assert.Equal(t, issues_model.PullRequestStatusConflict, conflictingPR.Status)
	// Ensure that mergeable returns false
	assert.False(t, conflictingPR.Mergeable(t.Context()))
}

func Test_PullRequestStatusCheckingCrossRepo_Mergeable_MergeTree(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, true)()
		testPullRequestStatusCheckingCrossRepoMergeable(t, giteaURL)
	})
}

func Test_PullRequestStatusCheckingCrossRepo_Mergeable_TmpRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, false)()
		testPullRequestStatusCheckingCrossRepoMergeable(t, giteaURL)
	})
}

func testPullRequestStatusCheckingCrossRepoMergeable(t *testing.T, giteaURL *url.URL) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)

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

	testRepoFork(t, session, baseRepo.OwnerName, baseRepo.Name, "org3", "conflict-checking", "main")

	forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "org3", Name: "conflict-checking"})

	// create a commit on new branch of forked repository
	testCreateFile(t, session, forkRepo.OwnerName, forkRepo.Name, "main", "important-secrets", "important_file", "Just a non-important file")

	// create Pull to merge the important-secrets branch into main branch.
	resp := testPullCreateDirectly(t, session, baseRepo.OwnerName, baseRepo.Name, "main",
		forkRepo.OwnerName, forkRepo.Name, "important-secrets", "PR with no conflict")
	// check the redirected URL
	url := test.RedirectURL(resp)
	assert.Regexp(t, fmt.Sprintf("^/%s/pulls/[0-9]*$", baseRepo.FullName()), url)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "PR with no conflict"})
	assert.NoError(t, issue.LoadPullRequest(t.Context()))
	conflictingPR := issue.PullRequest

	// Ensure conflictedFiles is populated.
	assert.Empty(t, conflictingPR.ConflictedFiles)
	// Check if status is correct.
	assert.Equal(t, issues_model.PullRequestStatusMergeable, conflictingPR.Status)
	// Ensure that mergeable returns true
	assert.True(t, conflictingPR.Mergeable(t.Context()))
}

func Test_PullRequestStatusCheckingCrossRepo_Conflicted_MergeTree(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, true)()
		testPullRequestStatusCheckingCrossRepoConflicted(t, giteaURL)
	})
}

func Test_PullRequestStatusCheckingCrossRepo_Conflicted_TmpRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, false)()
		testPullRequestStatusCheckingCrossRepoConflicted(t, giteaURL)
	})
}

func testPullRequestStatusCheckingCrossRepoConflicted(t *testing.T, giteaURL *url.URL) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)

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

	testRepoFork(t, session, baseRepo.OwnerName, baseRepo.Name, "org3", "conflict-checking", "main")

	forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "org3", Name: "conflict-checking"})

	// create a commit on new branch of forked repository
	testCreateFile(t, session, forkRepo.OwnerName, forkRepo.Name, "main", "important-secrets", "important_file", "Just a non-important file")

	// create a commit on main branch of base repository.
	testCreateFile(t, session, baseRepo.OwnerName, baseRepo.Name, "main", "main", "important_file", "Not the same content :P")

	// create Pull to merge the important-secrets branch into main branch.
	resp := testPullCreateDirectly(t, session, baseRepo.OwnerName, baseRepo.Name, "main",
		forkRepo.OwnerName, forkRepo.Name, "important-secrets", "PR with conflict!")
	// check the redirected URL
	url := test.RedirectURL(resp)
	assert.Regexp(t, fmt.Sprintf("^/%s/pulls/[0-9]*$", baseRepo.FullName()), url)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: "PR with conflict!"})
	assert.NoError(t, issue.LoadPullRequest(t.Context()))
	conflictingPR := issue.PullRequest

	// Ensure conflictedFiles is populated.
	assert.Equal(t, []string{"important_file"}, conflictingPR.ConflictedFiles)
	// Check if status is correct.
	assert.Equal(t, issues_model.PullRequestStatusConflict, conflictingPR.Status)
	// Ensure that mergeable returns false
	assert.False(t, conflictingPR.Mergeable(t.Context()))
}

func Test_PullRequest_AGit_StatusChecking_Mergeable_MergeTree(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// skip this test if git version is low
		if !git.DefaultFeatures().SupportProcReceive {
			return
		}

		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, true)()

		testPullRequestAGitStatusCheckingMergeable(t, giteaURL)
	})
}

func Test_PullRequest_AGit_StatusChecking_Mergeable_TmpRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, false)()

		testPullRequestAGitStatusCheckingMergeable(t, giteaURL)
	})
}

func testPullRequestAGitStatusCheckingMergeable(t *testing.T, giteaURL *url.URL) {
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

	// add something in local repository and push it to remote
	dstPath := t.TempDir()
	repoURL := *giteaURL
	repoURL.Path = baseRepo.FullName() + ".git"
	repoURL.User = url.UserPassword("user2", userPassword)
	doGitClone(dstPath, &repoURL)(t)

	gitRepo, err := git.OpenRepository(t.Context(), dstPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	doGitCreateBranch(dstPath, "test-agit-push")(t)

	_, err = generateCommitWithNewData(t.Context(), testFileSizeSmall, dstPath, "user2@example.com", "User Two", "branch-data-file-")
	assert.NoError(t, err)

	// push to create an agit pull request
	err = gitcmd.NewCommand("push", "origin",
		"-o", "title=agit-test-title", "-o", "description=agit-test-description",
		"-o", "topic=head-branch-name",
		"HEAD:refs/for/main",
	).WithDir(dstPath).Run(t.Context())
	assert.NoError(t, err)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
		RepoID: baseRepo.ID,
		Title:  "agit-test-title",
	})
	assert.NoError(t, issue.LoadPullRequest(t.Context()))
	conflictingPR := issue.PullRequest

	// Ensure conflictedFiles is populated.
	assert.Empty(t, conflictingPR.ConflictedFiles)
	// Check if status is correct.
	assert.Equal(t, issues_model.PullRequestStatusMergeable, conflictingPR.Status)
	// Ensure that mergeable returns true
	assert.True(t, conflictingPR.Mergeable(t.Context()))
}

func Test_PullRequest_AGit_StatusChecking_Conflicted_MergeTree(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, true)()

		testPullRequestAGitStatusCheckingConflicted(t, giteaURL)
	})
}

func Test_PullRequest_AGit_StatusChecking_Conflicted_TmpRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportGitMergeTree, false)()

		testPullRequestAGitStatusCheckingConflicted(t, giteaURL)
	})
}

func testPullRequestAGitStatusCheckingConflicted(t *testing.T, giteaURL *url.URL) {
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

	// add something in local repository and push it to remote
	dstPath := t.TempDir()
	repoURL := *giteaURL
	repoURL.Path = baseRepo.FullName() + ".git"
	repoURL.User = url.UserPassword("user2", userPassword)
	doGitClone(dstPath, &repoURL)(t)

	gitRepo, err := git.OpenRepository(t.Context(), dstPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// create agit branch from current commit
	doGitCreateBranch(dstPath, "test-agit-push")(t)

	doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
		LocalRepoPath:   dstPath,
		CheckoutBranch:  "main",
		TreeFilePath:    filepath.Join(dstPath, "README.md"),
		TreeFileContent: "Some changes to README file to main cause conflict",
	})

	err = gitcmd.NewCommand("push", "origin", "main").WithDir(dstPath).Run(t.Context())
	assert.NoError(t, err)

	// check out back to agit branch and change the same file
	doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
		LocalRepoPath:   dstPath,
		CheckoutBranch:  "test-agit-push",
		TreeFilePath:    filepath.Join(dstPath, "README.md"),
		TreeFileContent: "Some changes to README file for agit branch",
	})

	// push to create an agit pull request
	err = gitcmd.NewCommand("push", "origin",
		"-o", "title=agit-test-title", "-o", "description=agit-test-description",
		"-o", "topic=head-branch-name",
		"HEAD:refs/for/main",
	).WithDir(dstPath).Run(t.Context())
	assert.NoError(t, err)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
		RepoID: baseRepo.ID,
		Title:  "agit-test-title",
	})
	assert.NoError(t, issue.LoadPullRequest(t.Context()))
	conflictingPR := issue.PullRequest

	// Ensure conflictedFiles is populated.
	assert.Equal(t, []string{"README.md"}, conflictingPR.ConflictedFiles)
	// Check if status is correct.
	assert.Equal(t, issues_model.PullRequestStatusConflict, conflictingPR.Status)
	// Ensure that mergeable returns false
	assert.False(t, conflictingPR.Mergeable(t.Context()))
}
