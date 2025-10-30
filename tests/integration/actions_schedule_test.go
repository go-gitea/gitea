// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/migration"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	mirror_service "code.gitea.io/gitea/services/mirror"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/assert"
)

func TestScheduleUpdate(t *testing.T) {
	t.Run("Push", testScheduleUpdatePush)
	t.Run("PullMerge", testScheduleUpdatePullMerge)
	t.Run("DisableAndEnableActionsUnit", testScheduleUpdateDisableAndEnableActionsUnit)
	t.Run("ArchiveAndUnarchive", testScheduleUpdateArchiveAndUnarchive)
	t.Run("MirrorSync", testScheduleUpdateMirrorSync)
}

func testScheduleUpdatePush(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string) {
		newCron := "30 5 * * 1,3"
		pushScheduleChange(t, u, repo, newCron)
		branch, err := git_model.GetBranch(t.Context(), repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		return branch.CommitID, newCron
	})
}

func testScheduleUpdatePullMerge(t *testing.T) {
	newBranchName := "feat1"
	workflowTreePath := ".gitea/workflows/actions-schedule.yml"
	workflowContent := `name: actions-schedule
on:
  schedule:
    - cron:  '@every 2m' # update to 2m
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'schedule workflow'
`

	mergeStyles := []repo_model.MergeStyle{
		repo_model.MergeStyleMerge,
		repo_model.MergeStyleRebase,
		repo_model.MergeStyleRebaseMerge,
		repo_model.MergeStyleSquash,
		repo_model.MergeStyleFastForwardOnly,
	}

	for _, mergeStyle := range mergeStyles {
		t.Run(string(mergeStyle), func(t *testing.T) {
			doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string) {
				// update workflow file
				_, err := files_service.ChangeRepoFiles(t.Context(), repo, user, &files_service.ChangeRepoFilesOptions{
					NewBranch: newBranchName,
					Files: []*files_service.ChangeRepoFile{
						{
							Operation:     "update",
							TreePath:      workflowTreePath,
							ContentReader: strings.NewReader(workflowContent),
						},
					},
					Message: "update workflow schedule",
				})
				assert.NoError(t, err)

				// create pull request
				apiPull, err := doAPICreatePullRequest(testContext, repo.OwnerName, repo.Name, repo.DefaultBranch, newBranchName)(t)
				assert.NoError(t, err)

				// merge pull request
				testPullMerge(t, testContext.Session, repo.OwnerName, repo.Name, strconv.FormatInt(apiPull.Index, 10), MergeOptions{
					Style: mergeStyle,
				})

				pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
				return pull.MergedCommitID, "@every 2m"
			})
		})
	}

	t.Run(string(repo_model.MergeStyleManuallyMerged), func(t *testing.T) {
		doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string) {
			// enable manual-merge
			doAPIEditRepository(testContext, &api.EditRepoOption{
				HasPullRequests:  util.ToPointer(true),
				AllowManualMerge: util.ToPointer(true),
			})(t)

			// update workflow file
			fileResp, err := files_service.ChangeRepoFiles(t.Context(), repo, user, &files_service.ChangeRepoFilesOptions{
				NewBranch: newBranchName,
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      workflowTreePath,
						ContentReader: strings.NewReader(workflowContent),
					},
				},
				Message: "update workflow schedule",
			})
			assert.NoError(t, err)

			// merge and push
			dstPath := t.TempDir()
			u.Path = repo.FullName() + ".git"
			u.User = url.UserPassword(repo.OwnerName, userPassword)
			doGitClone(dstPath, u)(t)
			doGitMerge(dstPath, "origin/"+newBranchName)(t)
			doGitPushTestRepository(dstPath, "origin", repo.DefaultBranch)(t)

			// create pull request
			apiPull, err := doAPICreatePullRequest(testContext, repo.OwnerName, repo.Name, repo.DefaultBranch, newBranchName)(t)
			assert.NoError(t, err)

			// merge pull request manually
			doAPIManuallyMergePullRequest(testContext, repo.OwnerName, repo.Name, fileResp.Commit.SHA, apiPull.Index)(t)

			pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
			assert.Equal(t, issues_model.PullRequestStatusManuallyMerged, pull.Status)
			return pull.MergedCommitID, "@every 2m"
		})
	})
}

func testScheduleUpdateMirrorSync(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string) {
		// create mirror repo
		opts := migration.MigrateOptions{
			RepoName:    "actions-schedule-mirror",
			Description: "Test mirror for actions-schedule",
			Private:     false,
			Mirror:      true,
			CloneAddr:   repo.CloneLinkGeneral(t.Context()).HTTPS,
		}
		mirrorRepo, err := repo_service.CreateRepositoryDirectly(t.Context(), user, user, repo_service.CreateRepoOptions{
			Name:          opts.RepoName,
			Description:   opts.Description,
			IsPrivate:     opts.Private,
			IsMirror:      opts.Mirror,
			DefaultBranch: repo.DefaultBranch,
			Status:        repo_model.RepositoryBeingMigrated,
		}, false)
		assert.NoError(t, err)
		assert.True(t, mirrorRepo.IsMirror)
		mirrorRepo, err = repo_service.MigrateRepositoryGitData(t.Context(), user, mirrorRepo, opts, nil)
		assert.NoError(t, err)
		mirrorContext := NewAPITestContext(t, user.Name, mirrorRepo.Name, auth_model.AccessTokenScopeWriteRepository)

		// enable actions unit for mirror repo
		assert.False(t, mirrorRepo.UnitEnabled(t.Context(), unit_model.TypeActions))
		doAPIEditRepository(mirrorContext, &api.EditRepoOption{
			HasActions: util.ToPointer(true),
		})(t)
		actionSchedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: mirrorRepo.ID})
		scheduleSpec := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: mirrorRepo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, "@every 1m", scheduleSpec.Spec)

		// update remote repo
		newCron := "30 5,17 * * 2,4"
		pushScheduleChange(t, u, repo, newCron)
		repoDefaultBranch, err := git_model.GetBranch(t.Context(), repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)

		// sync
		ok := mirror_service.SyncPullMirror(t.Context(), mirrorRepo.ID)
		assert.True(t, ok)
		mirrorRepoDefaultBranch, err := git_model.GetBranch(t.Context(), mirrorRepo.ID, mirrorRepo.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, repoDefaultBranch.CommitID, mirrorRepoDefaultBranch.CommitID)

		// check updated schedule
		actionSchedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: mirrorRepo.ID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: mirrorRepo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, newCron, scheduleSpec.Spec)

		return repoDefaultBranch.CommitID, newCron
	})
}

func testScheduleUpdateArchiveAndUnarchive(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string) {
		doAPIEditRepository(testContext, &api.EditRepoOption{
			Archived: util.ToPointer(true),
		})(t)
		assert.Zero(t, unittest.GetCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}))
		doAPIEditRepository(testContext, &api.EditRepoOption{
			Archived: util.ToPointer(false),
		})(t)
		branch, err := git_model.GetBranch(t.Context(), repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		return branch.CommitID, "@every 1m"
	})
}

func testScheduleUpdateDisableAndEnableActionsUnit(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string) {
		doAPIEditRepository(testContext, &api.EditRepoOption{
			HasActions: util.ToPointer(false),
		})(t)
		assert.Zero(t, unittest.GetCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}))
		doAPIEditRepository(testContext, &api.EditRepoOption{
			HasActions: util.ToPointer(true),
		})(t)
		branch, err := git_model.GetBranch(t.Context(), repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)
		return branch.CommitID, "@every 1m"
	})
}

type scheduleUpdateTrigger func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (commitID, expectedSpec string)

func doTestScheduleUpdate(t *testing.T, updateTrigger scheduleUpdateTrigger) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-schedule", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		assert.NoError(t, repo.LoadAttributes(t.Context()))
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		wfTreePath := ".gitea/workflows/actions-schedule.yml"
		wfFileContent := `name: actions-schedule
on:
  schedule:
    - cron:  '@every 1m'
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'schedule workflow'
`

		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfTreePath, wfFileContent)
		apiFileResp := createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, opts1)

		actionSchedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID, CommitSHA: apiFileResp.Commit.SHA})
		scheduleSpec := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, "@every 1m", scheduleSpec.Spec)

		commitID, expectedSpec := updateTrigger(t, u, httpContext, user2, repo)

		actionSchedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID, CommitSHA: commitID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, expectedSpec, scheduleSpec.Spec)
	})
}

func pushScheduleChange(t *testing.T, u *url.URL, repo *repo_model.Repository, newCron string) {
	workflowTreePath := ".gitea/workflows/actions-schedule.yml"
	workflowContent := `name: actions-schedule
on:
  schedule:
    - cron:  '` + newCron + `'
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'schedule workflow'
`

	dstPath := t.TempDir()
	u.Path = repo.FullName() + ".git"
	u.User = url.UserPassword(repo.OwnerName, userPassword)
	doGitClone(dstPath, u)(t)
	doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
		LocalRepoPath:   dstPath,
		CheckoutBranch:  repo.DefaultBranch,
		TreeFilePath:    workflowTreePath,
		TreeFileContent: workflowContent,
	})(t)
	doGitPushTestRepository(dstPath, "origin", repo.DefaultBranch)(t)
}
