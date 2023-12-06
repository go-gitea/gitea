// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func TestPullRequestTargetEvent(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // owner of the base repo
		org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})  // owner of the forked repo

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

		// enable actions
		err = repo_model.UpdateRepositoryUnits(db.DefaultContext, baseRepo, []repo_model.RepoUnit{{
			RepoID: baseRepo.ID,
			Type:   unit_model.TypeActions,
		}}, nil)
		assert.NoError(t, err)

		// create the forked repo
		forkedRepo, err := repo_service.ForkRepository(git.DefaultContext, user2, org3, repo_service.ForkRepoOptions{
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
					Operation:     "create",
					TreePath:      ".gitea/workflows/pr.yml",
					ContentReader: strings.NewReader("name: test\non:\n  pull_request_target:\n    paths:\n      - 'file_*.txt'\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				Name:  user2.Name,
				Email: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				Name:  user2.Name,
				Email: user2.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, addWorkflowToBaseResp)

		// add a new file to the forked repo
		addFileToForkedResp, err := files_service.ChangeRepoFiles(git.DefaultContext, forkedRepo, org3, &files_service.ChangeRepoFilesOptions{
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
				Name:  org3.Name,
				Email: org3.Email,
			},
			Committer: &files_service.IdentityOptions{
				Name:  org3.Name,
				Email: org3.Email,
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
			PosterID: org3.ID,
			Poster:   org3,
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
		err = pull_service.NewPullRequest(git.DefaultContext, baseRepo, pullIssue, nil, nil, pullRequest, nil)
		assert.NoError(t, err)

		// load and compare ActionRun
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: baseRepo.ID}))
		actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID})
		assert.Equal(t, addFileToForkedResp.Commit.SHA, actionRun.CommitSHA)
		assert.Equal(t, actions_module.GithubEventPullRequestTarget, actionRun.TriggerEvent)

		// add another file whose name cannot match the specified path
		addFileToForkedResp, err = files_service.ChangeRepoFiles(git.DefaultContext, forkedRepo, org3, &files_service.ChangeRepoFilesOptions{
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
				Name:  org3.Name,
				Email: org3.Email,
			},
			Committer: &files_service.IdentityOptions{
				Name:  org3.Name,
				Email: org3.Email,
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
			PosterID: org3.ID,
			Poster:   org3,
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
		err = pull_service.NewPullRequest(git.DefaultContext, baseRepo, pullIssue, nil, nil, pullRequest, nil)
		assert.NoError(t, err)

		// the new pull request cannot trigger actions, so there is still only 1 record
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: baseRepo.ID}))
	})
}

func TestSkipCI(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:          "skip-ci",
			Description:   "test skip ci functionality",
			AutoInit:      true,
			Gitignores:    "Go",
			License:       "MIT",
			Readme:        "Default",
			DefaultBranch: "main",
			IsPrivate:     false,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, repo)

		// enable actions
		err = repo_model.UpdateRepositoryUnits(db.DefaultContext, repo, []repo_model.RepoUnit{{
			RepoID: repo.ID,
			Type:   unit_model.TypeActions,
		}}, nil)
		assert.NoError(t, err)

		// add workflow file to the repo
		addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      ".gitea/workflows/pr.yml",
					ContentReader: strings.NewReader("name: test\non:\n  push:\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
				},
			},
			Message:   "add workflow",
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				Name:  user2.Name,
				Email: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				Name:  user2.Name,
				Email: user2.Email,
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
			OldBranch: "main",
			NewBranch: "main",
			Author: &files_service.IdentityOptions{
				Name:  user2.Name,
				Email: user2.Email,
			},
			Committer: &files_service.IdentityOptions{
				Name:  user2.Name,
				Email: user2.Email,
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
	})
}
