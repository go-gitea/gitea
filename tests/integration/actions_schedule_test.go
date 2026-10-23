// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/migration"
	api "gitea.dev/modules/structs"
	actions_service "gitea.dev/services/actions"
	mirror_service "gitea.dev/services/mirror"
	repo_service "gitea.dev/services/repository"
	files_service "gitea.dev/services/repository/files"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleUpdate(t *testing.T) {
	t.Run("Push", testScheduleUpdatePush)
	t.Run("PullMerge", testScheduleUpdatePullMerge)
	t.Run("DisableAndEnableActionsUnit", testScheduleUpdateDisableAndEnableActionsUnit)
	t.Run("ArchiveAndUnarchive", testScheduleUpdateArchiveAndUnarchive)
	t.Run("MirrorSync", testScheduleUpdateMirrorSync)
	t.Run("ChangeDefaultBranch", testScheduleUpdateChangeDefaultBranch)
	t.Run("RenameDefaultBranch", testScheduleUpdateRenameDefaultBranch)
}

func testScheduleUpdatePush(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
		run := createScheduleRun(t, repo)
		newCron := "30 5 * * 1,3"
		pushScheduleChange(t, u, repo, newCron)
		assertRunStatus(t, run, actions_model.StatusWaiting)
		return newCron
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
			doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
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
				return "@every 2m"
			})
		})
	}

	t.Run(string(repo_model.MergeStyleManuallyMerged), func(t *testing.T) {
		doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
			// enable manual-merge
			doAPIEditRepository(testContext, &api.EditRepoOption{
				HasPullRequests:  new(true),
				AllowManualMerge: new(true),
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
			return "@every 2m"
		})
	})
}

func testScheduleUpdateMirrorSync(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
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
			HasActions: new(true),
		})(t)
		actionSchedule := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: mirrorRepo.ID})
		scheduleSpec := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: mirrorRepo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, "@every 1m", scheduleSpec.Spec)

		// update remote repo
		newCron := "30 5,17 * * 2,4"
		pushScheduleChange(t, u, repo, newCron)
		repoDefaultBranch, err := git_model.GetBranchExisting(t.Context(), repo.ID, repo.DefaultBranch)
		assert.NoError(t, err)

		// sync
		ok := mirror_service.SyncPullMirror(t.Context(), mirrorRepo.ID)
		assert.True(t, ok)
		mirrorRepoDefaultBranch, err := git_model.GetBranchExisting(t.Context(), mirrorRepo.ID, mirrorRepo.DefaultBranch)
		assert.NoError(t, err)
		assert.Equal(t, repoDefaultBranch.CommitID, mirrorRepoDefaultBranch.CommitID)

		// check updated schedule
		actionSchedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: mirrorRepo.ID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: mirrorRepo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, newCron, scheduleSpec.Spec)

		return newCron
	})
}

func testScheduleUpdateArchiveAndUnarchive(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
		run := createScheduleRun(t, repo)
		doAPIEditRepository(testContext, &api.EditRepoOption{
			Archived: new(true),
		})(t)
		assert.Zero(t, unittest.GetCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}))
		assertRunStatus(t, run, actions_model.StatusCancelled)
		doAPIEditRepository(testContext, &api.EditRepoOption{
			Archived: new(false),
		})(t)
		return "@every 1m"
	})
}

func testScheduleUpdateDisableAndEnableActionsUnit(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
		run := createScheduleRun(t, repo)
		doAPIEditRepository(testContext, &api.EditRepoOption{
			HasActions: new(true),
		})(t)
		assertRunStatus(t, run, actions_model.StatusWaiting)

		doAPIEditRepository(testContext, &api.EditRepoOption{
			HasActions: new(false),
		})(t)
		assert.Zero(t, unittest.GetCount(t, &actions_model.ActionSchedule{RepoID: repo.ID}))
		assertRunStatus(t, run, actions_model.StatusCancelled)
		doAPIEditRepository(testContext, &api.EditRepoOption{
			HasActions: new(true),
		})(t)
		return "@every 1m"
	})
}

func testScheduleUpdateChangeDefaultBranch(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
		oldBranch, newBranch := repo.DefaultBranch, "release"
		testAPICreateBranch(t, testContext.Session, repo.OwnerName, repo.Name, oldBranch, newBranch, http.StatusCreated)

		run := createScheduleRun(t, repo)
		req := NewRequestWithValues(t, "POST", "/"+repo.FullName()+"/settings/branches", map[string]string{
			"action": "default_branch",
			"branch": newBranch,
		})
		testContext.Session.MakeRequest(t, req, http.StatusSeeOther)
		assertRunStatus(t, run, actions_model.StatusCancelled)
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID, Ref: git.RefNameFromBranch(newBranch).String()})

		run = createScheduleRun(t, repo)
		doAPIEditRepository(testContext, &api.EditRepoOption{
			DefaultBranch: new(oldBranch),
		})(t)
		assertRunStatus(t, run, actions_model.StatusCancelled)
		return "@every 1m"
	})
}

func testScheduleUpdateRenameDefaultBranch(t *testing.T) {
	doTestScheduleUpdate(t, func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string) {
		newName := repo.DefaultBranch + "-renamed"
		run := createScheduleRun(t, repo)
		testAPIRenameBranch(t, user.Name, repo.OwnerName, repo.Name, repo.DefaultBranch, newName, http.StatusNoContent)
		assertRunStatus(t, run, actions_model.StatusCancelled)
		return "@every 1m"
	})
}

type scheduleUpdateTrigger func(t *testing.T, u *url.URL, testContext APITestContext, user *user_model.User, repo *repo_model.Repository) (expectedSpec string)

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

		expectedSpec := updateTrigger(t, u, httpContext, user2, repo)

		defaultBranch := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID}).DefaultBranch
		branch, err := git_model.GetBranchExisting(t.Context(), repo.ID, defaultBranch)
		require.NoError(t, err)
		actionSchedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{RepoID: repo.ID, Ref: git.RefNameFromBranch(defaultBranch).String(), CommitSHA: branch.CommitID})
		scheduleSpec = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID, ScheduleID: actionSchedule.ID})
		assert.Equal(t, expectedSpec, scheduleSpec.Spec)
	})
}

func createScheduleRun(t *testing.T, repo *repo_model.Repository) *actions_model.ActionRun {
	spec := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID})
	spec.Repo = repo
	spec.Schedule = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionSchedule{ID: spec.ScheduleID})
	require.NoError(t, actions_service.CreateScheduleTaskBySpec(t.Context(), spec))
	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, ScheduleID: spec.ScheduleID})
	assert.Equal(t, actions_model.StatusWaiting, run.Status)
	return run
}

func assertRunStatus(t *testing.T, run *actions_model.ActionRun, status actions_model.Status) {
	assert.Equal(t, status, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID}).Status)
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
