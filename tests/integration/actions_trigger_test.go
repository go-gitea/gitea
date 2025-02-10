// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/timeutil"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
	release_service "code.gitea.io/gitea/services/release"
	repo_service "code.gitea.io/gitea/services/repository"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func TestPullRequestTargetEvent(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of the base repo
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // owner of the forked repo

		// create the base repo
		baseRepo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "repo-pull-request-target",
			Description:   "test pull-request-target event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, baseRepo)

		// add user4 as the collaborator
		ctx := NewAPITestContext(t, baseRepo.OwnerName, baseRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		t.Run("AddUser4AsCollaboratorWithReadAccess", doAPIAddCollaborator(ctx, "user4", perm.AccessModeRead))

		// create the forked repo
		forkedRepo, err := repo_service.ForkRepository(git.DefaultContext, user2, user4, repo_service.ForkRepoOptions{
			BaseRepo:    baseRepo,
			Name:        "forked-repo-pull-request-target",
			Description: "test pull-request-target event",
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, forkedRepo)

		// add workflow file to the base repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, baseRepo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/pr.yml",
					ContentReader: strings.NewReader(`name: test
on:
  pull_request_target:
    paths:
      - 'file_*.txt'
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// add a new file to the forked repo
		addFileToForkedResp, err := files_service.ChangeRepoFiles(git.DefaultContext, forkedRepo, user4, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "file_1.txt",
					ContentReader: strings.NewReader("file1"),
				},
			},
			Message:   "add file1",
			OldBranch: "main",
			NewBranch: "fork-branch-1",
			Author: &files_service.IdentityOptions{
				GitUserName:  user4.Name,
				GitUserEmail: user4.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user4.Name,
				GitUserEmail: user4.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addFileToForkedResp)

		// create Pull
		pullIssue := &issues_model.Issue{
			RepoID:   baseRepo.ID,
			Title:    "Test pull-request-target-event",
			PosterID: user4.ID,
			Poster:   user4,
			IsPull:   true,
		}
		pullRequest := &issues_model.PullRequest{
			HeadRepoID: forkedRepo.ID,
			BaseRepoID: baseRepo.ID,
			HeadBranch: "fork-branch-1",
			BaseBranch: "main",
			HeadRepo:   forkedRepo,
			BaseRepo:   baseRepo,
			Type:       issues_model.PullRequestGitea,
		}
		prOpts := &pull_service.NewPullRequestOptions{Repo: baseRepo, Issue: pullIssue, PullRequest: pullRequest}
		err = pull_service.NewPullRequest(git.DefaultContext, prOpts)
		assert.NoError(t, err)

		// load and compare ActionRun
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: baseRepo.ID}))
		actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID})
		assert.Equal(t, addFileToForkedResp.Commit.SHA, actionRun.CommitSHA)
		assert.Equal(t, actions_module.GithubEventPullRequestTarget, actionRun.TriggerEvent)

		// add another file whose name cannot match the specified path
		addFileToForkedResp, err = files_service.ChangeRepoFiles(git.DefaultContext, forkedRepo, user4, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "foo.txt",
					ContentReader: strings.NewReader("foo"),
				},
			},
			Message:   "add foo.txt",
			OldBranch: "main",
			NewBranch: "fork-branch-2",
			Author: &files_service.IdentityOptions{
				GitUserName:  user4.Name,
				GitUserEmail: user4.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user4.Name,
				GitUserEmail: user4.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addFileToForkedResp)

		// create Pull
		pullIssue = &issues_model.Issue{
			RepoID:   baseRepo.ID,
			Title:    "A mismatched path cannot trigger pull-request-target-event",
			PosterID: user4.ID,
			Poster:   user4,
			IsPull:   true,
		}
		pullRequest = &issues_model.PullRequest{
			HeadRepoID: forkedRepo.ID,
			BaseRepoID: baseRepo.ID,
			HeadBranch: "fork-branch-2",
			BaseBranch: "main",
			HeadRepo:   forkedRepo,
			BaseRepo:   baseRepo,
			Type:       issues_model.PullRequestGitea,
		}
		prOpts = &pull_service.NewPullRequestOptions{Repo: baseRepo, Issue: pullIssue, PullRequest: pullRequest}
		err = pull_service.NewPullRequest(git.DefaultContext, prOpts)
		assert.NoError(t, err)

		// the new pull request cannot trigger actions, so there is still only 1 record
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: baseRepo.ID}))
	})
}

func TestSkipCI(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "skip-ci",
			Description:   "test skip ci functionality",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "master",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/pr.yml",
					ContentReader: strings.NewReader(`name: test
on:
  push:
    branches: [master]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "master",
			NewBranch: "master",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// a run has been created
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))

		// add a file with a configured skip-ci string in commit message
		addFileResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "bar.txt",
					ContentReader: strings.NewReader("bar"),
				},
			},
			Message:   fmt.Sprintf("%s add bar", setting.Actions.SkipWorkflowStrings[0]),
			OldBranch: "master",
			NewBranch: "master",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addFileResp)

		// the commit message contains a configured skip-ci string, so there is still only 1 record
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))

		// add file to new branch
		addFileToBranchResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "test-skip-ci",
					ContentReader: strings.NewReader("test-skip-ci"),
				},
			},
			Message:   "add test file",
			OldBranch: "master",
			NewBranch: "test-skip-ci",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addFileToBranchResp)

		resp := testPullCreate(t, session, "user2", "skip-ci", true, "master", "test-skip-ci", "[skip ci] test-skip-ci")

		// check the redirected URL
		url := test.RedirectURL(resp)
		assert.Regexp(t, "^/user2/skip-ci/pulls/[0-9]*$", url)

		// the pr title contains a configured skip-ci string, so there is still only 1 record
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))
	})
}

func TestCreateDeleteRefEvent(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "create-delete-ref-event",
			Description:   "test create delete ref ci event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/createdelete.yml",
					ContentReader: strings.NewReader(`name: test
on:
  [create,delete]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// Get the commit ID of the default branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		branch, err := git_model.GetBranch(db.DefaultContext, repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)

		// create a branch
		err = repo_service.CreateNewBranchFromCommit(db.DefaultContext, user2, repo, gitRepo, branch.CommitID, "test-create-branch")
		assert.NoError(t, err)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "create",
			Ref:        "refs/heads/test-create-branch",
			WorkflowID: "createdelete.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)

		// create a tag
		err = release_service.CreateNewTag(db.DefaultContext, user2, repo, branch.CommitID, "test-create-tag", "test create tag event")
		assert.NoError(t, err)
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "create",
			Ref:        "refs/tags/test-create-tag",
			WorkflowID: "createdelete.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)

		// delete the branch
		err = repo_service.DeleteBranch(db.DefaultContext, user2, repo, gitRepo, "test-create-branch", nil)
		assert.NoError(t, err)
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "delete",
			Ref:        "refs/heads/main",
			WorkflowID: "createdelete.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)

		// delete the tag
		tag, err := repo_model.GetRelease(db.DefaultContext, repo.ID, "test-create-tag")
		assert.NoError(t, err)
		err = release_service.DeleteReleaseByID(db.DefaultContext, repo, tag, user2, true)
		assert.NoError(t, err)
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "delete",
			Ref:        "refs/heads/main",
			WorkflowID: "createdelete.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)
	})
}

func TestPullRequestCommitStatusEvent(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of the repo
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // contributor of the repo

		// create a repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "repo-pull-request",
			Description:   "test pull-request event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add user4 as the collaborator
		ctx := NewAPITestContext(t, repo.OwnerName, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		t.Run("AddUser4AsCollaboratorWithReadAccess", doAPIAddCollaborator(ctx, "user4", perm.AccessModeRead))

		// add the workflow file to the repo
		addWorkflow, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/pr.yml",
					ContentReader: strings.NewReader(`name: test
on:
  pull_request:
    types: [assigned, unassigned, labeled, unlabeled, opened, edited, closed, reopened, synchronize, milestoned, demilestoned, review_requested, review_request_removed]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflow)
		sha := addWorkflow.Commit.SHA

		// create a new branch
		testBranch := "test-branch"
		gitRepo, err := git.OpenRepository(git.DefaultContext, ".")
		assert.NoError(t, err)
		err = repo_service.CreateNewBranch(git.DefaultContext, user2, repo, gitRepo, "main", testBranch)
		assert.NoError(t, err)

		// create Pull
		pullIssue := &issues_model.Issue{
			RepoID:   repo.ID,
			Title:    "A test PR",
			PosterID: user2.ID,
			Poster:   user2,
			IsPull:   true,
		}
		pullRequest := &issues_model.PullRequest{
			HeadRepoID: repo.ID,
			BaseRepoID: repo.ID,
			HeadBranch: testBranch,
			BaseBranch: "main",
			HeadRepo:   repo,
			BaseRepo:   repo,
			Type:       issues_model.PullRequestGitea,
		}
		prOpts := &pull_service.NewPullRequestOptions{Repo: repo, Issue: pullIssue, PullRequest: pullRequest}
		err = pull_service.NewPullRequest(db.DefaultContext, prOpts)
		assert.NoError(t, err)

		// opened
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// edited
		err = issue_service.ChangeContent(db.DefaultContext, pullIssue, user2, "test", 0)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// closed
		err = issue_service.CloseIssue(db.DefaultContext, pullIssue, user2, "")
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// reopened
		err = issue_service.ReopenIssue(db.DefaultContext, pullIssue, user2, "")
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// assign
		removed, _, err := issue_service.ToggleAssigneeWithNotify(db.DefaultContext, pullIssue, user2, user4.ID)
		assert.False(t, removed)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// unassign
		removed, _, err = issue_service.ToggleAssigneeWithNotify(db.DefaultContext, pullIssue, user2, user4.ID)
		assert.True(t, removed)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// labeled
		label := &issues_model.Label{
			RepoID:      repo.ID,
			Name:        "test",
			Exclusive:   false,
			Description: "test",
			Color:       "#e11d21",
		}
		err = issues_model.NewLabel(db.DefaultContext, label)
		assert.NoError(t, err)
		err = issue_service.AddLabel(db.DefaultContext, pullIssue, user2, label)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// unlabeled
		err = issue_service.RemoveLabel(db.DefaultContext, pullIssue, user2, label)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// synchronize
		addFileResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "test.txt",
					ContentReader: strings.NewReader("test"),
				},
			},
			Message:   "add file",
			OldBranch: testBranch,
			NewBranch: testBranch,
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addFileResp)
		sha = addFileResp.Commit.SHA
		assert.Eventually(t, func() bool {
			latestCommitStatuses, _, err := git_model.GetLatestCommitStatus(db.DefaultContext, repo.ID, sha, db.ListOptionsAll)
			assert.NoError(t, err)
			if len(latestCommitStatuses) == 0 {
				return false
			}
			if latestCommitStatuses[0].State == api.CommitStatusPending {
				insertFakeStatus(t, repo, sha, latestCommitStatuses[0].TargetURL, latestCommitStatuses[0].Context)
				return true
			}
			return false
		}, 1*time.Second, 100*time.Millisecond)

		// milestoned
		milestone := &issues_model.Milestone{
			RepoID:       repo.ID,
			Name:         "test",
			Content:      "test",
			DeadlineUnix: timeutil.TimeStampNow(),
		}
		err = issues_model.NewMilestone(db.DefaultContext, milestone)
		assert.NoError(t, err)
		err = issue_service.ChangeMilestoneAssign(db.DefaultContext, pullIssue, user2, milestone.ID)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// demilestoned
		err = issue_service.ChangeMilestoneAssign(db.DefaultContext, pullIssue, user2, milestone.ID)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// review_requested
		_, err = issue_service.ReviewRequest(db.DefaultContext, pullIssue, user2, nil, user4, true)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)

		// review_request_removed
		_, err = issue_service.ReviewRequest(db.DefaultContext, pullIssue, user2, nil, user4, false)
		assert.NoError(t, err)
		checkCommitStatusAndInsertFakeStatus(t, repo, sha)
	})
}

func checkCommitStatusAndInsertFakeStatus(t *testing.T, repo *repo_model.Repository, sha string) {
	latestCommitStatuses, _, err := git_model.GetLatestCommitStatus(db.DefaultContext, repo.ID, sha, db.ListOptionsAll)
	assert.NoError(t, err)
	assert.Len(t, latestCommitStatuses, 1)
	assert.Equal(t, api.CommitStatusPending, latestCommitStatuses[0].State)

	insertFakeStatus(t, repo, sha, latestCommitStatuses[0].TargetURL, latestCommitStatuses[0].Context)
}

func insertFakeStatus(t *testing.T, repo *repo_model.Repository, sha, targetURL, context string) {
	err := commitstatus_service.CreateCommitStatus(db.DefaultContext, repo, user_model.NewActionsUser(), sha, &git_model.CommitStatus{
		State:     api.CommitStatusSuccess,
		TargetURL: targetURL,
		Context:   context,
	})
	assert.NoError(t, err)
}

func TestWorkflowDispatchPublicApi(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "workflow-dispatch-event",
			Description:   "test workflow-dispatch ci event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// Get the commit ID of the default branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		branch, err := git_model.GetBranch(db.DefaultContext, repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		values := url.Values{}
		values.Set("ref", "main")
		req := NewRequestWithURLValues(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), values).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "workflow_dispatch",
			Ref:        "refs/heads/main",
			WorkflowID: "dispatch.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)
	})
}

func TestWorkflowDispatchPublicApiWithInputs(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "workflow-dispatch-event",
			Description:   "test workflow-dispatch ci event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch: { inputs: { myinput: { default: def }, myinput2: { default: def2 }, myinput3: { type: boolean, default: false } } }
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// Get the commit ID of the default branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		branch, err := git_model.GetBranch(db.DefaultContext, repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		values := url.Values{}
		values.Set("ref", "main")
		values.Set("inputs[myinput]", "val0")
		values.Set("inputs[myinput3]", "true")
		req := NewRequestWithURLValues(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), values).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "workflow_dispatch",
			Ref:        "refs/heads/main",
			WorkflowID: "dispatch.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)
		dispatchPayload := &api.WorkflowDispatchPayload{}
		err = json.Unmarshal([]byte(run.EventPayload), dispatchPayload)
		assert.NoError(t, err)
		assert.Contains(t, dispatchPayload.Inputs, "myinput")
		assert.Contains(t, dispatchPayload.Inputs, "myinput2")
		assert.Contains(t, dispatchPayload.Inputs, "myinput3")
		assert.Equal(t, "val0", dispatchPayload.Inputs["myinput"])
		assert.Equal(t, "def2", dispatchPayload.Inputs["myinput2"])
		assert.Equal(t, "true", dispatchPayload.Inputs["myinput3"])
	})
}

func TestWorkflowDispatchPublicApiJSON(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "workflow-dispatch-event",
			Description:   "test workflow-dispatch ci event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch: { inputs: { myinput: { default: def }, myinput2: { default: def2 }, myinput3: { type: boolean, default: false } } }
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// Get the commit ID of the default branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		branch, err := git_model.GetBranch(db.DefaultContext, repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		inputs := &api.CreateActionWorkflowDispatch{
			Ref: "main",
			Inputs: map[string]string{
				"myinput":  "val0",
				"myinput3": "true",
			},
		}

		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), inputs).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "workflow_dispatch",
			Ref:        "refs/heads/main",
			WorkflowID: "dispatch.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)
	})
}

func TestWorkflowDispatchPublicApiWithInputsJSON(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "workflow-dispatch-event",
			Description:   "test workflow-dispatch ci event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch: { inputs: { myinput: { default: def }, myinput2: { default: def2 }, myinput3: { type: boolean, default: false } } }
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// Get the commit ID of the default branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		branch, err := git_model.GetBranch(db.DefaultContext, repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		inputs := &api.CreateActionWorkflowDispatch{
			Ref: "main",
			Inputs: map[string]string{
				"myinput":  "val0",
				"myinput3": "true",
			},
		}
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), inputs).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "workflow_dispatch",
			Ref:        "refs/heads/main",
			WorkflowID: "dispatch.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)
		dispatchPayload := &api.WorkflowDispatchPayload{}
		err = json.Unmarshal([]byte(run.EventPayload), dispatchPayload)
		assert.NoError(t, err)
		assert.Contains(t, dispatchPayload.Inputs, "myinput")
		assert.Contains(t, dispatchPayload.Inputs, "myinput2")
		assert.Contains(t, dispatchPayload.Inputs, "myinput3")
		assert.Equal(t, "val0", dispatchPayload.Inputs["myinput"])
		assert.Equal(t, "def2", dispatchPayload.Inputs["myinput2"])
		assert.Equal(t, "true", dispatchPayload.Inputs["myinput3"])
	})
}

func TestWorkflowDispatchPublicApiWithInputsNonDefaultBranchJSON(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "workflow-dispatch-event",
			Description:   "test workflow-dispatch ci event",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// add workflow file to the repo
		addWorkflowToBaseResp, err = files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "update",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch: { inputs: { myinput: { default: def }, myinput2: { default: def2 }, myinput3: { type: boolean, default: false } } }
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "dispatch",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// Get the commit ID of the dispatch branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		commit, err := gitRepo.GetBranchCommit("dispatch")
		assert.NoError(t, err)
		inputs := &api.CreateActionWorkflowDispatch{
			Ref: "refs/heads/dispatch",
			Inputs: map[string]string{
				"myinput":  "val0",
				"myinput3": "true",
			},
		}
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), inputs).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "workflow_dispatch",
			Ref:        "refs/heads/dispatch",
			WorkflowID: "dispatch.yml",
			CommitSHA:  commit.ID.String(),
		})
		assert.NotNil(t, run)
		dispatchPayload := &api.WorkflowDispatchPayload{}
		err = json.Unmarshal([]byte(run.EventPayload), dispatchPayload)
		assert.NoError(t, err)
		assert.Contains(t, dispatchPayload.Inputs, "myinput")
		assert.Contains(t, dispatchPayload.Inputs, "myinput2")
		assert.Contains(t, dispatchPayload.Inputs, "myinput3")
		assert.Equal(t, "val0", dispatchPayload.Inputs["myinput"])
		assert.Equal(t, "def2", dispatchPayload.Inputs["myinput2"])
		assert.Equal(t, "true", dispatchPayload.Inputs["myinput3"])
	})
}

func TestWorkflowApi(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "workflow-api",
			Description:   "test workflow apis",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/workflows", repo.FullName())).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		workflows := &api.ActionWorkflowResponse{}
		json.NewDecoder(resp.Body).Decode(workflows)
		assert.Empty(t, workflows.Workflows)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".gitea/workflows/dispatch.yml",
					ContentReader: strings.NewReader(`name: test
on:
  workflow_dispatch: { inputs: { myinput: { default: def }, myinput2: { default: def2 }, myinput3: { type: boolean, default: false } } }
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo helloworld
`),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				GitUserName:  user2.Name,
				GitUserEmail: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/workflows", repo.FullName())).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		json.NewDecoder(resp.Body).Decode(workflows)
		assert.Len(t, workflows.Workflows, 1)
		assert.Equal(t, "dispatch.yml", workflows.Workflows[0].Name)
		assert.Equal(t, ".gitea/workflows/dispatch.yml", workflows.Workflows[0].Path)
		assert.Equal(t, ".gitea/workflows/dispatch.yml", workflows.Workflows[0].Path)
		assert.Equal(t, "active", workflows.Workflows[0].State)

		// Use a hardcoded api path
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/%s", repo.FullName(), workflows.Workflows[0].ID)).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		workflow := &api.ActionWorkflow{}
		json.NewDecoder(resp.Body).Decode(workflow)
		assert.Equal(t, workflows.Workflows[0].ID, workflow.ID)
		assert.Equal(t, workflows.Workflows[0].Path, workflow.Path)
		assert.Equal(t, workflows.Workflows[0].URL, workflow.URL)
		assert.Equal(t, workflows.Workflows[0].HTMLURL, workflow.HTMLURL)
		assert.Equal(t, workflows.Workflows[0].Name, workflow.Name)
		assert.Equal(t, workflows.Workflows[0].State, workflow.State)

		// Use the provided url instead of the hardcoded one
		req = NewRequest(t, "GET", workflows.Workflows[0].URL).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		workflow = &api.ActionWorkflow{}
		json.NewDecoder(resp.Body).Decode(workflow)
		assert.Equal(t, workflows.Workflows[0].ID, workflow.ID)
		assert.Equal(t, workflows.Workflows[0].Path, workflow.Path)
		assert.Equal(t, workflows.Workflows[0].URL, workflow.URL)
		assert.Equal(t, workflows.Workflows[0].HTMLURL, workflow.HTMLURL)
		assert.Equal(t, workflows.Workflows[0].Name, workflow.Name)
		assert.Equal(t, workflows.Workflows[0].State, workflow.State)

		// Disable the workflow
		req = NewRequest(t, "PUT", workflows.Workflows[0].URL+"/disable").
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		// Use the provided url instead of the hardcoded one
		req = NewRequest(t, "GET", workflows.Workflows[0].URL).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		workflow = &api.ActionWorkflow{}
		json.NewDecoder(resp.Body).Decode(workflow)
		assert.Equal(t, workflows.Workflows[0].ID, workflow.ID)
		assert.Equal(t, workflows.Workflows[0].Path, workflow.Path)
		assert.Equal(t, workflows.Workflows[0].URL, workflow.URL)
		assert.Equal(t, workflows.Workflows[0].HTMLURL, workflow.HTMLURL)
		assert.Equal(t, workflows.Workflows[0].Name, workflow.Name)
		assert.Equal(t, "disabled_manually", workflow.State)

		inputs := &api.CreateActionWorkflowDispatch{
			Ref: "main",
			Inputs: map[string]string{
				"myinput":  "val0",
				"myinput3": "true",
			},
		}
		// Since the workflow is disabled, so the response code is 403 forbidden
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), inputs).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusForbidden)

		// Enable the workflow again
		req = NewRequest(t, "PUT", workflows.Workflows[0].URL+"/enable").
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		// Use the provided url instead of the hardcoded one
		req = NewRequest(t, "GET", workflows.Workflows[0].URL).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		workflow = &api.ActionWorkflow{}
		json.NewDecoder(resp.Body).Decode(workflow)
		assert.Equal(t, workflows.Workflows[0].ID, workflow.ID)
		assert.Equal(t, workflows.Workflows[0].Path, workflow.Path)
		assert.Equal(t, workflows.Workflows[0].URL, workflow.URL)
		assert.Equal(t, workflows.Workflows[0].HTMLURL, workflow.HTMLURL)
		assert.Equal(t, workflows.Workflows[0].Name, workflow.Name)
		assert.Equal(t, workflows.Workflows[0].State, workflow.State)

		req = NewRequest(t, "GET", workflows.Workflows[0].URL).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		workflow = &api.ActionWorkflow{}
		json.NewDecoder(resp.Body).Decode(workflow)
		assert.Equal(t, workflows.Workflows[0].ID, workflow.ID)
		assert.Equal(t, workflows.Workflows[0].Path, workflow.Path)
		assert.Equal(t, workflows.Workflows[0].URL, workflow.URL)
		assert.Equal(t, workflows.Workflows[0].HTMLURL, workflow.HTMLURL)
		assert.Equal(t, workflows.Workflows[0].Name, workflow.Name)
		assert.Equal(t, workflows.Workflows[0].State, workflow.State)

		// Get the commit ID of the default branch
		gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
		assert.NoError(t, err)
		defer gitRepo.Close()
		branch, err := git_model.GetBranch(db.DefaultContext, repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		inputs = &api.CreateActionWorkflowDispatch{
			Ref: "main",
			Inputs: map[string]string{
				"myinput":  "val0",
				"myinput3": "true",
			},
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/workflows/dispatch.yml/dispatches", repo.FullName()), inputs).
			AddTokenAuth(token)
		_ = MakeRequest(t, req, http.StatusNoContent)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			Title:      "add workflow",
			RepoID:     repo.ID,
			Event:      "workflow_dispatch",
			Ref:        "refs/heads/main",
			WorkflowID: "dispatch.yml",
			CommitSHA:  branch.CommitID,
		})
		assert.NotNil(t, run)
		dispatchPayload := &api.WorkflowDispatchPayload{}
		err = json.Unmarshal([]byte(run.EventPayload), dispatchPayload)
		assert.NoError(t, err)
		assert.Contains(t, dispatchPayload.Inputs, "myinput")
		assert.Contains(t, dispatchPayload.Inputs, "myinput2")
		assert.Contains(t, dispatchPayload.Inputs, "myinput3")
		assert.Equal(t, "val0", dispatchPayload.Inputs["myinput"])
		assert.Equal(t, "def2", dispatchPayload.Inputs["myinput2"])
		assert.Equal(t, "true", dispatchPayload.Inputs["myinput3"])
	})
}
