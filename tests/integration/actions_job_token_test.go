// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	actions_service "code.gitea.io/gitea/services/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsJobTokenPermissiveAccess(t *testing.T) {
	cases := []struct {
		name   string
		isFork bool

		ownerPermMode repo_model.ActionsTokenPermissionMode
		ownerMaxPerms map[unit_model.Type]perm.AccessMode

		repoPermMode repo_model.ActionsTokenPermissionMode
		repoMaxPerms map[unit_model.Type]perm.AccessMode

		expectGitAccess perm.AccessMode
	}{
		{
			name:            "SameRepo-Permissive",
			repoPermMode:    repo_model.ActionsTokenPermissionModePermissive,
			expectGitAccess: perm.AccessModeWrite,
		},
		{
			name:            "SameRepo-Restricted",
			repoPermMode:    repo_model.ActionsTokenPermissionModeRestricted,
			expectGitAccess: perm.AccessModeRead,
		},
		{
			name:            "Fork-Permissive",
			repoPermMode:    repo_model.ActionsTokenPermissionModePermissive,
			isFork:          true,
			expectGitAccess: perm.AccessModeRead,
		},
		{
			name:            "Fork-Restricted",
			repoPermMode:    repo_model.ActionsTokenPermissionModeRestricted,
			isFork:          true,
			expectGitAccess: perm.AccessModeRead,
		},
	}
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})

		ownerActionsCfg, err := actions_model.GetOwnerActionsConfig(t.Context(), task.OwnerID)
		require.NoError(t, err)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: task.RepoID})
		repoActionsUnit := repo.MustGetUnit(t.Context(), unit_model.TypeActions)
		repoActionsCfg := repoActionsUnit.ActionsConfig()

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				// prepare owner's token permissions settings
				ownerActionsCfg.TokenPermissionMode = tt.ownerPermMode
				ownerActionsCfg.MaxTokenPermissions = util.Iif(tt.ownerMaxPerms == nil, nil, &repo_model.ActionsTokenPermissions{UnitAccessModes: tt.ownerMaxPerms})
				require.NoError(t, actions_model.SetOwnerActionsConfig(t.Context(), task.OwnerID, ownerActionsCfg))

				// prepare repo's token permissions settings
				repoActionsCfg.OverrideOwnerConfig = tt.repoPermMode != "" || tt.repoMaxPerms != nil
				repoActionsCfg.TokenPermissionMode = tt.repoPermMode
				repoActionsCfg.MaxTokenPermissions = util.Iif(tt.repoMaxPerms == nil, nil, &repo_model.ActionsTokenPermissions{UnitAccessModes: tt.repoMaxPerms})
				require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), repoActionsUnit))

				// prepare task and its token
				require.NoError(t, task.GenerateToken())
				task.Status = actions_model.StatusRunning
				task.IsForkPullRequest = tt.isFork
				err := actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status", "is_fork_pull_request")
				require.NoError(t, err)

				require.NoError(t, task.LoadJob(t.Context()))
				require.NoError(t, task.Job.LoadRun(t.Context()))
				task.Job.Run.IsForkPullRequest = tt.isFork
				require.NoError(t, actions_model.UpdateRun(t.Context(), task.Job.Run, "is_fork_pull_request"))

				apiCtx := APITestContext{
					Session:  emptyTestSession(t),
					Token:    task.Token,
					Username: "user5",
					Reponame: "repo4",
				}

				testURL := *u
				testURL.User = url.UserPassword("gitea-actions", task.Token)

				// can read git content
				testURL.Path = "/user5/repo4.git/HEAD"
				MakeRequest(t, NewRequest(t, "GET", testURL.String()), http.StatusOK)

				// can read git-lfs content
				testURL.Path = "/user5/repo4.git/info/lfs/locks"
				req := NewRequest(t, "GET", testURL.String()).SetHeader("Accept", lfs.MediaType)
				MakeRequest(t, req, http.StatusOK)

				// can write git-lfs content
				testURL.Path = "/user5/repo4.git/info/lfs/objects/batch"
				req = NewRequestWithJSON(t, "POST", testURL.String(), lfs.BatchRequest{Operation: "upload"}).SetHeader("Accept", lfs.MediaType)
				MakeRequest(t, req, util.Iif(tt.expectGitAccess == perm.AccessModeWrite, http.StatusOK, http.StatusUnauthorized))

				// write access should be forbidden for fork PRs even in permissive mode
				apiCtx.ExpectedCode = util.Iif(tt.expectGitAccess == perm.AccessModeWrite, http.StatusCreated, http.StatusForbidden)
				t.Run("CreateBranchFile", doAPICreateFile(apiCtx, "test-file", &structs.CreateFileOptions{
					FileOptions:   structs.FileOptions{NewBranchName: "new-branch" + t.Name()},
					ContentBase64: base64.StdEncoding.EncodeToString([]byte(`dummy content`)),
				}))

				// no other permissions
				apiCtx.ExpectedCode = http.StatusForbidden
				t.Run("ForbidCreateRepository", doAPIDeleteRepository(apiCtx))
			})
		}
	})
}

func TestActionsTokenPermissionsClamping(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Create Repo
		apiRepo := createActionsTestRepo(t, token, "repo-clamping", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		// Mock Runner
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// Set Clamping Config: Permissive Mode, Max Code = Read
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", repo.OwnerName, repo.Name), map[string]string{
			"token_permission_mode":  "permissive",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_code":               "read",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Create workflow requesting Write
		wfTreePath := ".gitea/workflows/clamping.yml"
		wfFileContent := `name: Clamping
on: [push]
permissions:
  contents: write
jobs:
  job-clamping:
    runs-on: ubuntu-latest
    steps:
      - run: echo test
`
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfTreePath, wfFileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, opts)

		// Fetch task
		runnerTask := runner.fetchTask(t)
		taskToken := runnerTask.Secrets["GITEA_TOKEN"]
		require.NotEmpty(t, taskToken)

		// Verify Permissions
		testCtx := APITestContext{
			Session:  emptyTestSession(t),
			Token:    taskToken,
			Username: user2.Name,
			Reponame: repo.Name,
		}

		// 1. Try to Write (Create File) - Should Fail (403) because Max is Read
		testCtx.ExpectedCode = http.StatusForbidden
		t.Run("Fail to Create File (Max Clamping)", doAPICreateFile(testCtx, "clamping.txt", &structs.CreateFileOptions{
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("test")),
		}))

		// 2. Try to Read (Get Repository) - Should Succeed (200)
		testCtx.ExpectedCode = http.StatusOK
		t.Run("Get Repository (Read Allowed)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, repo.Name, r.Name)
		}))
	})
}

func TestActionsCrossRepoAccess(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)

		// 1. Create Organization
		orgName := "org-cross-test"
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &structs.CreateOrgOption{
			UserName: orgName,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		owner, err := org_model.GetOrgByName(t.Context(), orgName)
		require.NoError(t, err)

		// 2. Create Two Repositories in owner
		createRepoInOrg := func(name string) int64 {
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &structs.CreateRepoOption{
				Name:     name,
				AutoInit: true,
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			var repo structs.Repository
			DecodeJSON(t, resp, &repo)
			return repo.ID
		}

		repoAID := createRepoInOrg("repo-A")
		repoBID := createRepoInOrg("repo-B")

		// 3. Enable Actions in Repo A (Source) and Repo B (Target)
		enableActions := func(repoID int64) {
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repoID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
				},
			})
			require.NoError(t, err)
		}

		enableActions(repoAID)
		enableActions(repoBID)

		// 4. Create Task in Repo A, and use A's token to access B
		taskA := createActionTask(t, repoAID, false)
		testCtxA := APITestContext{
			Session:  emptyTestSession(t),
			Token:    taskA.Token,
			Username: orgName,
			Reponame: "repo-B",
		}

		testCtxA.ExpectedCode = http.StatusOK
		t.Run("PublicCrossRepoAccess", doAPIGetRepository(testCtxA, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-B", r.Name)
		}))

		// make repo-B be private
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", orgName, "repo-B"), &structs.EditRepoOption{
			Private: new(true),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		testCtxA.ExpectedCode = http.StatusNotFound
		t.Run("NoPrivateCrossRepoAccess", doAPIGetRepository(testCtxA, nil))

		ownerActionsCfg := actions_model.OwnerActionsConfig{AllowedCrossRepoIDs: []int64{repoBID}}
		require.NoError(t, actions_model.SetOwnerActionsConfig(t.Context(), owner.ID, ownerActionsCfg))

		testCtxA.ExpectedCode = http.StatusOK
		t.Run("AccessToSelectedPrivateRepo", doAPIGetRepository(testCtxA, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-B", r.Name)
		}))

		t.Run("RepoTransfer", func(t *testing.T) {
			ownerActionsCfg, err := actions_model.GetOwnerActionsConfig(t.Context(), owner.ID)
			require.NoError(t, err)
			assert.Contains(t, ownerActionsCfg.AllowedCrossRepoIDs, repoBID)

			// Transfer Repository to user4
			req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/repo-B/transfer", orgName), &structs.TransferRepoOption{
				NewOwner: "user4",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			// Accept transfer as user4
			session4 := loginUser(t, "user4")
			token4 := getTokenForLoggedInUser(t, session4, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
			req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/repo-B/transfer/accept", orgName)).AddTokenAuth(token4)
			MakeRequest(t, req, http.StatusAccepted)

			// Verify it is removed from the org's config
			ownerActionsCfg, err = actions_model.GetOwnerActionsConfig(t.Context(), owner.ID)
			require.NoError(t, err)
			assert.NotContains(t, ownerActionsCfg.AllowedCrossRepoIDs, repoBID)
		})
	})
}

func TestActionsTokenPermissionsWorkflowScenario(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Step 1: Create a new repository with Actions enabled
		httpContext := NewAPITestContext(t, "user2", "repo-workflow-token-test", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
		t.Run("Create Repository and Test Token Permissions", doAPICreateRepository(httpContext, false, func(t *testing.T, repository structs.Repository) {
			// Step 2: Enable Actions unit with Permissive mode (the mode the reviewer set)
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repository.ID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
					OverrideOwnerConfig: true,
					// No MaxTokenPermissions - allows full write access
				},
			})
			require.NoError(t, err)

			// Step 3: Create an Actions task (simulates a running workflow)
			task := createActionTask(t, repository.ID, false)

			// Step 4: Use the GITEA_TOKEN to create a file via API (exactly as the reviewer's workflow did)
			session := emptyTestSession(t)
			testCtx := APITestContext{
				Session:  session,
				Token:    task.Token,
				Username: "user2",
				Reponame: "repo-workflow-token-test",
			}

			// The create file should succeed with permissive mode
			testCtx.ExpectedCode = http.StatusCreated
			t.Run("GITEA_TOKEN Create File (Permissive Mode)", doAPICreateFile(testCtx, fmt.Sprintf("test-file-%d.txt", time.Now().Unix()), &structs.CreateFileOptions{
				FileOptions: structs.FileOptions{
					BranchName: "master",
					Message:    "test actions token",
				},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("Test Content")),
			}))

			// Verify that the API also works for reading (should always work)
			testCtx.ExpectedCode = http.StatusOK
			t.Run("GITEA_TOKEN Get Repository", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
				assert.Equal(t, "repo-workflow-token-test", r.Name)
			}))

			// Now test with Restricted mode - file creation should fail
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repository.ID})
			actionsUnit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
			require.NoError(t, err)
			actionsCfg := actionsUnit.ActionsConfig()
			actionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionModeRestricted
			actionsUnit.Config = actionsCfg
			require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), actionsUnit))

			// Regenerate token to get fresh permissions
			require.NoError(t, task.GenerateToken())
			task.Status = actions_model.StatusRunning
			require.NoError(t, actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status"))

			testCtx.Token = task.Token
			testCtx.ExpectedCode = http.StatusForbidden
			t.Run("GITEA_TOKEN Create File (Restricted Mode - Should Fail)", doAPICreateFile(testCtx, "should-fail.txt", &structs.CreateFileOptions{
				FileOptions: structs.FileOptions{
					BranchName: "master",
					Message:    "this should fail",
				},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("Should Not Be Created")),
			}))
		}))
	})
}

// TestActionsWorkflowPermissionsKeyword tests that the `permissions:` keyword in a workflow YAML
// restricts the token even when the repository is in permissive mode.
func TestActionsWorkflowPermissionsKeyword(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		httpContext := NewAPITestContext(t, "user2", "repo-workflow-perms-kw", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
		t.Run("Workflow Permissions Keyword", doAPICreateRepository(httpContext, false, func(t *testing.T, repository structs.Repository) {
			// Enable Actions unit with PERMISSIVE mode (default write access)
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repository.ID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
				},
			})
			require.NoError(t, err)

			// Define Workflow YAML with two jobs:
			// 1. job-read-only: Inherits `permissions: read-all` (write should fail)
			// 2. job-override: Overrides with `permissions: contents: write` (write should succeed)
			workflowYAML := `
name: Test Permissions
on: workflow_dispatch
permissions: read-all

jobs:
  job-read-only:
    runs-on: ubuntu-latest
    steps:
      - run: echo "Full read-only"

  job-none-perms:
    permissions: none
    runs-on: ubuntu-latest
    steps:
      - run: echo "Full read-only"

  job-override:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - run: echo "Override to write"
`
			// Parse the workflow using the actual parsing logic (this verifies the parser works as expected)
			singleWorkflows, err := jobparser.Parse([]byte(workflowYAML))
			require.NoError(t, err)
			// jobparser.Parse returns one SingleWorkflow per job

			// Get default permissions for the repo (Permissive)
			repo, err := repo_model.GetRepositoryByID(t.Context(), repository.ID)
			require.NoError(t, err)
			actionsUnit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
			require.NoError(t, err)
			cfg := actionsUnit.ActionsConfig()
			defaultPerms := cfg.GetDefaultTokenPermissions()

			// Create Run (shared)
			run := &actions_model.ActionRun{
				RepoID:        repository.ID,
				OwnerID:       repository.Owner.ID,
				Title:         "Test workflow permissions",
				Status:        actions_model.StatusRunning,
				Ref:           "refs/heads/master",
				CommitSHA:     "abc123456",
				TriggerUserID: repository.Owner.ID,
			}
			require.NoError(t, db.Insert(t.Context(), run))

			// Iterate over jobs and create them matching the parser logic
			for _, flow := range singleWorkflows {
				jobID, jobDef := flow.Job()
				jobName := jobDef.Name

				// Use the combined explicit extraction logic
				explicitPerms := actions_service.ExtractJobPermissionsFromWorkflow(flow, jobDef)
				var finalPerms repo_model.ActionsTokenPermissions
				if explicitPerms != nil {
					finalPerms = *explicitPerms
				} else {
					finalPerms = defaultPerms
				}
				finalPerms = cfg.ClampPermissions(finalPerms)

				job := &actions_model.ActionRunJob{
					RunID:            run.ID,
					RepoID:           repository.ID,
					OwnerID:          repository.Owner.ID,
					CommitSHA:        "abc123456",
					Name:             jobName,
					JobID:            jobID,
					Status:           actions_model.StatusRunning,
					TokenPermissions: &finalPerms,
				}
				require.NoError(t, db.Insert(t.Context(), job))

				task := &actions_model.ActionTask{
					JobID:             job.ID,
					RepoID:            repository.ID,
					Status:            actions_model.StatusRunning,
					IsForkPullRequest: false,
				}
				require.NoError(t, task.GenerateToken())
				require.NoError(t, db.Insert(t.Context(), task))

				// Link task to job
				job.TaskID = task.ID
				_, err = db.GetEngine(t.Context()).ID(job.ID).Cols("task_id").Update(job)
				require.NoError(t, err)

				// Test API Access
				session := emptyTestSession(t)
				testCtx := APITestContext{
					Session:  session,
					Token:    task.Token,
					Username: "user2",
					Reponame: "repo-workflow-perms-kw",
				}

				if jobID == "job-read-only" {
					// Should match 'read-all' -> Write Forbidden
					testCtx.ExpectedCode = http.StatusForbidden
					t.Run("Job [read-only] Create File (Should Fail)", doAPICreateFile(testCtx, "fail-readonly.txt", &structs.CreateFileOptions{
						ContentBase64: base64.StdEncoding.EncodeToString([]byte("fail")),
					}))

					testCtx.ExpectedCode = http.StatusOK
					t.Run("Job [read-only] Get Repo (Should Succeed)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
						assert.Equal(t, repository.Name, r.Name)
					}))
				} else if jobID == "job-none-perms" {
					// Should match 'none' -> Read/Write Forbidden (404 for private repo)
					testCtx.ExpectedCode = http.StatusNotFound
					t.Run("Job [none] Create File (Should Fail)", doAPICreateFile(testCtx, "fail-none.txt", &structs.CreateFileOptions{
						ContentBase64: base64.StdEncoding.EncodeToString([]byte("fail")),
					}))

					testCtx.ExpectedCode = http.StatusNotFound
					t.Run("Job [none] Get Repo (Should Fail)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
						// Should not reach here if 404
					}))
				} else if jobID == "job-override" {
					// Should have 'contents: write' -> Write Created
					testCtx.ExpectedCode = http.StatusCreated
					t.Run("Job [override] Create File (Should Succeed)", doAPICreateFile(testCtx, "succeed-override.txt", &structs.CreateFileOptions{
						FileOptions: structs.FileOptions{
							BranchName: "master",
							Message:    "override success",
						},
						ContentBase64: base64.StdEncoding.EncodeToString([]byte("success")),
					}))
				}
			}
		}))
	})
}

func TestActionsPermission(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// create a new repo
		apiRepo := createActionsTestRepo(t, token, "actions-permission", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		// create a mock runner
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// set actions token permission mode to "permissive"
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", repo.OwnerName, repo.Name), map[string]string{
			"token_permission_mode": "permissive",
			"override_owner_config": "true",
		})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		require.Equal(t, fmt.Sprintf("/%s/%s/settings/actions/general", repo.OwnerName, repo.Name), test.RedirectURL(resp))

		// create a workflow file with "permission" keyword
		wfTreePath := ".gitea/workflows/test_permissions.yml"
		wfFileContent := `name: Test Permissions
on:
  push:
    paths:
      - '.gitea/workflows/test_permissions.yml'

permissions: read-all

jobs:
  job-override:
    runs-on: ubuntu-latest
    permissions: write-all
    steps:
      - run: echo "Override to write"
`
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfTreePath, wfFileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, opts)

		// fetch a task(*runnerv1.Task) and get its token
		runnerTask := runner.fetchTask(t)
		taskToken := runnerTask.Secrets["GITEA_TOKEN"]
		require.NotEmpty(t, taskToken)
		// get the task(*actions_model.ActionTask) by token
		task, err := actions_model.GetRunningTaskByToken(t.Context(), taskToken)
		require.NoError(t, err)
		require.Equal(t, repo.ID, task.RepoID)
		require.False(t, task.IsForkPullRequest)
		require.Equal(t, actions_model.StatusRunning, task.Status)
		actionsPerm, err := access_model.GetActionsUserRepoPermission(t.Context(), repo, user_model.NewActionsUser(), task.ID)
		require.NoError(t, err)
		require.NoError(t, task.LoadJob(t.Context()))
		t.Logf("Computed Units Mode: %+v", actionsPerm)
		require.True(t, actionsPerm.CanWrite(unit_model.TypeCode), "Should have write access to Code. Got: %v", actionsPerm.AccessMode) // the token should have the "write" permission on "Code" unit
		// test creating a file with the token
		actionsTokenContext := APITestContext{
			Session:      emptyTestSession(t),
			Token:        taskToken,
			Username:     repo.OwnerName,
			Reponame:     repo.Name,
			ExpectedCode: 0,
		}
		t.Run("API Create File", doAPICreateFile(actionsTokenContext, "test-permissions.txt", &structs.CreateFileOptions{
			FileOptions: structs.FileOptions{
				BranchName: repo.DefaultBranch,
				Message:    "Create File",
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte(`This is a test file for permissions.`)),
		}, func(t *testing.T, resp structs.FileResponse) {
			require.NotEmpty(t, resp.Content.SHA)
		}))
	})
}

func createActionTask(t *testing.T, repoID int64, isFork bool) *actions_model.ActionTask {
	run := &actions_model.ActionRun{
		RepoID:            repoID,
		Status:            actions_model.StatusRunning,
		IsForkPullRequest: isFork,
		WorkflowID:        "test.yaml",
		TriggerUserID:     1,
		Ref:               "refs/heads/main",
		CommitSHA:         "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:             "push",
		TriggerEvent:      "push",
	}

	index, err := db.GetNextResourceIndex(t.Context(), "action_run_index", repoID)
	require.NoError(t, err)
	run.Index = index

	require.NoError(t, db.Insert(t.Context(), run))

	job := &actions_model.ActionRunJob{
		RunID:             run.ID,
		RepoID:            repoID,
		Status:            actions_model.StatusRunning,
		IsForkPullRequest: isFork,
		JobID:             "test_job",
		Name:              "test_job",
	}
	require.NoError(t, db.Insert(t.Context(), job))

	task := &actions_model.ActionTask{
		JobID:             job.ID,
		RepoID:            repoID,
		Status:            actions_model.StatusRunning,
		IsForkPullRequest: isFork,
	}
	require.NoError(t, task.GenerateToken())
	require.NoError(t, db.Insert(t.Context(), task))

	job.TaskID = task.ID
	_, err = actions_model.UpdateRunJob(t.Context(), job, nil, "task_id")
	require.NoError(t, err)

	task.Job = job
	return task
}

func TestActionsOverrideOwnerConfig(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)

		orgName := "org-override-test"
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &structs.CreateOrgOption{
			UserName: orgName,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		org, err := user_model.GetUserByName(t.Context(), orgName)
		require.NoError(t, err)

		// 1. Set Org Config to RESTRICTED and Max=Read
		orgCfg := actions_model.OwnerActionsConfig{
			TokenPermissionMode: repo_model.ActionsTokenPermissionModeRestricted,
			MaxTokenPermissions: &repo_model.ActionsTokenPermissions{
				UnitAccessModes: map[unit_model.Type]perm.AccessMode{
					unit_model.TypeIssues: perm.AccessModeRead,
					unit_model.TypeCode:   perm.AccessModeRead,
				},
			},
		}
		require.NoError(t, actions_model.SetOwnerActionsConfig(t.Context(), org.ID, orgCfg))

		// 2. Create Repository in Org
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &structs.CreateRepoOption{
			Name:     "repo-override",
			AutoInit: true,
			Private:  true,
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		var apiRepo structs.Repository
		DecodeJSON(t, resp, &apiRepo)
		repoID := apiRepo.ID

		enableActions := func(override bool, mode repo_model.ActionsTokenPermissionMode) {
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repoID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					OverrideOwnerConfig: override,
					TokenPermissionMode: mode,
					MaxTokenPermissions: &repo_model.ActionsTokenPermissions{
						UnitAccessModes: map[unit_model.Type]perm.AccessMode{
							unit_model.TypeIssues: perm.AccessModeWrite, // Repo tries to be more permissive than org
							unit_model.TypeCode:   perm.AccessModeWrite,
						},
					},
				},
			})
			require.NoError(t, err)
		}

		// 3. Test OverrideOwnerConfig = false (Owner config clamping should apply)
		enableActions(false, repo_model.ActionsTokenPermissionModePermissive)
		task1 := createActionTask(t, repoID, false)

		testCtx := APITestContext{
			Session:  emptyTestSession(t),
			Token:    task1.Token,
			Username: orgName,
			Reponame: "repo-override",
		}

		// Write should FAIL because Override=false means Owner config (Max=READ, Default=RESTRICTED) is used
		testCtx.ExpectedCode = http.StatusForbidden
		t.Run("Override=False Owner Clamping (Should Fail Write)", doAPICreateFile(testCtx, "fail-file.txt", &structs.CreateFileOptions{
			FileOptions: structs.FileOptions{
				BranchName: "master",
				Message:    "fail write test",
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("fail")),
		}))

		// Read should SUCCEED
		testCtx.ExpectedCode = http.StatusOK
		t.Run("Override=False Owner Clamping (Should Succeed Read)", doAPIGetRepository(testCtx, nil))

		// Clean up the repo unit to reset
		_, err = db.DeleteByBean(t.Context(), &repo_model.RepoUnit{RepoID: repoID, Type: unit_model.TypeActions})
		require.NoError(t, err)

		// 4. Test OverrideOwnerConfig = true (Repo config should apply, clamping bypassed)
		enableActions(true, repo_model.ActionsTokenPermissionModePermissive)
		task2 := createActionTask(t, repoID, false)
		testCtx.Token = task2.Token

		// Write should SUCCEED because Override=true bypasses Owner restrictions
		testCtx.ExpectedCode = http.StatusCreated
		t.Run("Override=True Repo Config (Should Succeed Write)", doAPICreateFile(testCtx, "success-file.txt", &structs.CreateFileOptions{
			FileOptions: structs.FileOptions{
				BranchName: "master",
				Message:    "success write test",
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("success")),
		}))

		// 5. Test empty permissions mapping `permissions: {}` (None/Restricted)
		// We simulate this by overriding the specific job permissions in the DB to match what the parser does for `permissions: {}`
		// which results in all None.
		task3 := createActionTask(t, repoID, false)
		job, err := actions_model.GetRunJobByRepoAndID(t.Context(), task3.RepoID, task3.JobID)
		require.NoError(t, err)

		job.TokenPermissions = &repo_model.ActionsTokenPermissions{}
		_, err = db.GetEngine(t.Context()).ID(job.ID).Cols("token_permissions").Update(job)
		require.NoError(t, err)
		require.NoError(t, task3.GenerateToken())
		require.NoError(t, actions_model.UpdateTask(t.Context(), task3, "token_hash", "token_salt", "token_last_eight"))

		testCtx.Token = task3.Token
		// Read should FAIL because permissions: {} sets EVERYTHING to none
		testCtx.ExpectedCode = http.StatusNotFound
		t.Run("Empty Permissions Mapping (Should Fail Read)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
			// Should not reach here
		}))
	})
}

func TestActionsTokenPermissionsExceedsTargetRepoLimit(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// create repos
		repo1 := createActionsTestRepo(t, token, "actions-permission-repo1", false)
		repo2 := createActionsTestRepo(t, token, "actions-permission-repo2", true)

		// set owner-level actions config to "selected" and add repo2
		req := NewRequestWithValues(t, "POST", "/user/settings/actions/general", map[string]string{
			"cross_repo_add_target":      "true",
			"cross_repo_add_target_name": repo2.Name,
		})
		session.MakeRequest(t, req, http.StatusOK)
		// create the runner for repo1
		runner1 := newMockRunner()
		runner1.registerAsRepoRunner(t, user2.Name, repo1.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// set actions token permission mode to "permissive" for repo1
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo1.Name), map[string]string{
			"token_permission_mode": "permissive",
			"override_owner_config": "true",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		// set actions token permission mode to "restricted" for repo2 and set max permissions
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo2.Name), map[string]string{
			"token_permission_mode":  "restricted",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeIssues)): "read",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// create a workflow file with "permission" keyword for repo1
		wfTreePath := ".gitea/workflows/test_permissions.yml"
		wfFileContent := `name: Test Permissions
on:
  push:
    paths:
      - '.gitea/workflows/test_permissions.yml'

jobs:
  job-override:
    runs-on: ubuntu-latest
    permissions: write-all
    steps:
      - run: echo "test perms"
`
		opts := getWorkflowCreateFileOptions(user2, repo1.DefaultBranch, "create "+wfTreePath, wfFileContent)
		createWorkflowFile(t, token, user2.Name, repo1.Name, wfTreePath, opts)

		task1 := runner1.fetchTask(t)
		task1Token := task1.Secrets["GITEA_TOKEN"]
		require.NotEmpty(t, task1Token)

		// should fail: target repo does not allow code access
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo2.Name)).AddTokenAuth(task1Token)
		MakeRequest(t, req, http.StatusNotFound)

		// set "max_code" to "read" so that the actions token can access code
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/token_permissions", user2.Name, repo2.Name), map[string]string{
			"token_permission_mode":  "restricted",
			"override_owner_config":  "true",
			"enable_max_permissions": "true",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeCode)):   "read",
			"max_unit_access_mode_" + strconv.Itoa(int(unit_model.TypeIssues)): "read",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// should succeed: target repo now allows code read access for this token
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo2.Name)).AddTokenAuth(task1Token)
		MakeRequest(t, req, http.StatusOK)
	})
}
