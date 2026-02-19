// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"
	actions_service "code.gitea.io/gitea/services/actions"

	"github.com/nektos/act/pkg/jobparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsJobTokenAccess(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("Write Access", testActionsJobTokenAccess(u, false))
		t.Run("Read Access", testActionsJobTokenAccess(u, true))
	})
}

func testActionsJobTokenAccess(u *url.URL, isFork bool) func(t *testing.T) {
	return func(t *testing.T) {
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})

		// Ensure the Actions unit exists for the repository with default permissive mode
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: task.RepoID})
		actionsUnit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
		if repo_model.IsErrUnitTypeNotExist(err) {
			// Insert Actions unit if it doesn't exist
			err = db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{},
			})
			require.NoError(t, err)
		} else {
			require.NoError(t, err)
			// Ensure permissive mode for this test
			actionsCfg := actionsUnit.ActionsConfig()
			actionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionModePermissive
			actionsCfg.MaxTokenPermissions = nil
			actionsUnit.Config = actionsCfg
			require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), actionsUnit))
		}

		require.NoError(t, task.GenerateToken())
		task.Status = actions_model.StatusRunning
		task.IsForkPullRequest = isFork
		err = actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status", "is_fork_pull_request")
		require.NoError(t, err)

		// Also update the run for fork clamping check
		if isFork {
			require.NoError(t, task.LoadJob(t.Context()))
			require.NoError(t, task.Job.LoadRun(t.Context()))
			task.Job.Run.IsForkPullRequest = true
			require.NoError(t, actions_model.UpdateRun(t.Context(), task.Job.Run, "is_fork_pull_request"))
		}

		session := emptyTestSession(t)
		context := APITestContext{
			Session:  session,
			Token:    task.Token,
			Username: "user5",
			Reponame: "repo4",
		}
		dstPath := t.TempDir()

		u.Path = context.GitPath()
		u.User = url.UserPassword("gitea-actions", task.Token)

		t.Run("Git Clone", doGitClone(dstPath, u))

		t.Run("API Get Repository", doAPIGetRepository(context, func(t *testing.T, r structs.Repository) {
			require.Equal(t, "repo4", r.Name)
			require.Equal(t, "user5", r.Owner.UserName)
		}))

		context.ExpectedCode = util.Iif(isFork, http.StatusForbidden, http.StatusCreated)
		t.Run("API Create File", doAPICreateFile(context, "test.txt", &structs.CreateFileOptions{
			FileOptions: structs.FileOptions{
				NewBranchName: "new-branch",
				Message:       "Create File",
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte(`This is a test file created using job token.`)),
		}))

		context.ExpectedCode = http.StatusForbidden
		t.Run("Fail to Create Repository", doAPICreateRepository(context, true))

		context.ExpectedCode = http.StatusForbidden
		t.Run("Fail to Delete Repository", doAPIDeleteRepository(context))

		t.Run("Fail to Create Organization", doAPICreateOrganization(context, &structs.CreateOrgOption{
			UserName: "actions",
			FullName: "Gitea Actions",
		}))
	}
}

func TestActionsJobTokenAccessLFS(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		httpContext := NewAPITestContext(t, "user2", "repo-lfs-test", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
		t.Run("Create Repository", doAPICreateRepository(httpContext, false, func(t *testing.T, repository structs.Repository) {
			task := createActionTask(t, repository.ID, false)

			// Enable Actions unit for the repository
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repository.ID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{},
			})
			require.NoError(t, err)
			session := emptyTestSession(t)
			httpContext := APITestContext{
				Session:  session,
				Token:    task.Token,
				Username: "user2",
				Reponame: "repo-lfs-test",
			}

			u.Path = httpContext.GitPath()
			dstPath := t.TempDir()

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword("gitea-actions", task.Token)

			t.Run("Clone", doGitClone(dstPath, u))

			dstPath2 := t.TempDir()

			t.Run("Partial Clone", doPartialGitClone(dstPath2, u))

			lfs := lfsCommitAndPushTest(t, dstPath, testFileSizeSmall)[0]

			reqLFS := NewRequest(t, "GET", "/api/v1/repos/user2/repo-lfs-test/media/"+lfs).AddTokenAuth(task.Token)
			respLFS := MakeRequestNilResponseRecorder(t, reqLFS, http.StatusOK)
			assert.Equal(t, testFileSizeSmall, respLFS.Length)
		}))
	})
}

func TestActionsTokenPermissionsModes(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("Permissive Mode (default)", testActionsTokenPermissionsMode(u, "permissive", false))
		t.Run("Restricted Mode", testActionsTokenPermissionsMode(u, "restricted", true))
	})
}

func testActionsTokenPermissionsMode(u *url.URL, mode string, expectReadOnly bool) func(t *testing.T) {
	return func(t *testing.T) {
		// Update repository settings to the requested mode
		if mode != "" {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: "repo4", OwnerName: "user5"})
			require.NoError(t, repo.LoadUnits(t.Context()))
			actionsUnit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
			require.NoError(t, err, "Actions unit should exist for repo4")
			actionsCfg := actionsUnit.ActionsConfig()
			actionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionMode(mode)
			actionsCfg.MaxTokenPermissions = nil // Ensure no max permissions interfere
			// Update the config
			actionsUnit.Config = actionsCfg
			require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), actionsUnit))
		}

		// Load a task that can be used for testing
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
		// Regenerate token to pick up new permissions if any (though currently permissions are checked at runtime)
		require.NoError(t, task.GenerateToken())
		task.Status = actions_model.StatusRunning
		task.IsForkPullRequest = false // Not a fork PR
		err := actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status", "is_fork_pull_request")
		require.NoError(t, err)

		session := emptyTestSession(t)
		context := APITestContext{
			Session:  session,
			Token:    task.Token,
			Username: "user5",
			Reponame: "repo4",
		}
		dstPath := t.TempDir()

		u.Path = context.GitPath()
		u.User = url.UserPassword("gitea-actions", task.Token)

		// Git clone should always work (read access)
		t.Run("Git Clone", doGitClone(dstPath, u))

		// API Get should always work (read access)
		t.Run("API Get Repository", doAPIGetRepository(context, func(t *testing.T, r structs.Repository) {
			require.Equal(t, "repo4", r.Name)
			require.Equal(t, "user5", r.Owner.UserName)
		}))

		var sha string

		// Test Write Access
		if expectReadOnly {
			context.ExpectedCode = http.StatusForbidden
		} else {
			context.ExpectedCode = 0
		}
		t.Run("API Create File", doAPICreateFile(context, "test-permissions.txt", &structs.CreateFileOptions{
			FileOptions: structs.FileOptions{
				BranchName: "master",
				Message:    "Create File",
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte(`This is a test file for permissions.`)),
		}, func(t *testing.T, resp structs.FileResponse) {
			sha = resp.Content.SHA
			require.NotEmpty(t, sha, "SHA should not be empty")
		}))

		// Test Delete Access
		if expectReadOnly {
			context.ExpectedCode = http.StatusForbidden
		} else {
			context.ExpectedCode = 0
		}
		if !expectReadOnly {
			// Clean up created file if we had write access
			t.Run("API Delete File", func(t *testing.T) {
				t.Logf("Deleting file with SHA: %s", sha)
				require.NotEmpty(t, sha, "SHA must be captured before deletion")
				deleteOpts := &structs.DeleteFileOptions{
					FileOptions: structs.FileOptions{
						BranchName: "master",
						Message:    "Delete File",
					},
					SHA: sha,
				}
				req := NewRequestWithJSON(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", context.Username, context.Reponame, "test-permissions.txt"), deleteOpts).
					AddTokenAuth(context.Token)
				if context.ExpectedCode != 0 {
					context.Session.MakeRequest(t, req, context.ExpectedCode)
					return
				}
				context.Session.MakeRequest(t, req, http.StatusOK)
			})
		}
	}
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

		// 2. Create Two Repositories in Org
		createRepoInOrg := func(name string) int64 {
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &structs.CreateRepoOption{
				Name:     name,
				AutoInit: true,
				Private:  true, // Must be private for potential restrictions
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

		// 4. Create Task in Repo A
		task := createActionTask(t, repoAID, false)

		// 5. Verify Access to Repo B (Target)
		testCtx := APITestContext{
			Session:  emptyTestSession(t),
			Token:    task.Token,
			Username: orgName,
			Reponame: "repo-B",
		}

		// Case A: Default (AllowCrossRepoAccess = true by default now) -> Should Succeed (200) Read-Only
		// API returns 404 if denied (hidden), 200 if allowed.
		testCtx.ExpectedCode = http.StatusOK
		t.Run("Cross-Repo Access Allowed (Default)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-B", r.Name)
		}))

		// Case B: Explicitly Disable AllowCrossRepoAccess
		org, err := org_model.GetOrgByName(t.Context(), orgName)
		require.NoError(t, err)

		cfg := &repo_model.ActionsConfig{
			CrossRepoMode: repo_model.ActionsCrossRepoModeNone,
		}
		err = actions_model.SetOrgActionsConfig(t.Context(), org.ID, cfg)
		require.NoError(t, err)

		// Retry -> Should Fail (404 Not Found)
		testCtx.ExpectedCode = http.StatusNotFound
		t.Run("Cross-Repo Access Denied (Disabled)", doAPIGetRepository(testCtx, nil))

		// Case C: Public Repository Access -> Should Succeed even if cross-repo is disabled
		bFalse := false
		repoCID := createRepoInOrg("repo-C")
		// Make it public via API
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", orgName, "repo-C"), &structs.EditRepoOption{
			Private: &bFalse,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
		enableActions(repoCID)

		testCtxC := APITestContext{
			Session:      emptyTestSession(t),
			Token:        task.Token,
			Username:     orgName,
			Reponame:     "repo-C",
			ExpectedCode: http.StatusOK,
		}
		t.Run("Cross-Repo Access Allowed for Public Repo (Disabled Policy)", doAPIGetRepository(testCtxC, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-C", r.Name)
		}))

		// 6. Test Cross-Repo Package Access
		t.Run("Cross-Repo Package Access", func(t *testing.T) {
			packageName := "cross-test-pkg"
			packageVersion := "1.0.0"
			fileName := "test-file.bin"
			content := []byte{1, 2, 3, 4, 5}

			// First, upload a package to the org using basic auth (user2 is org owner)
			packageURL := fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", orgName, packageName, packageVersion, fileName)
			uploadReq := NewRequestWithBody(t, "PUT", packageURL, bytes.NewReader(content)).AddBasicAuth("user2")
			MakeRequest(t, uploadReq, http.StatusCreated)

			// Link the package to repo-B (per reviewer feedback: packages must be linked to repos)
			pkg, err := packages_model.GetPackageByName(t.Context(), org.ID, packages_model.TypeGeneric, packageName)
			require.NoError(t, err)
			require.NoError(t, packages_model.SetRepositoryLink(t.Context(), pkg.ID, repoBID))

			// By default, cross-repo is disabled
			// Explicitly set it to false to ensure test determinism (in case defaults change)
			require.NoError(t, actions_model.SetOrgActionsConfig(t.Context(), org.ID, &repo_model.ActionsConfig{
				CrossRepoMode: repo_model.ActionsCrossRepoModeNone,
			}))

			// FIXME use private repository
			// // Try to download with cross-repo disabled - should fail
			// downloadReqDenied := NewRequest(t, "GET", packageURL)
			// downloadReqDenied.Header.Set("Authorization", "Bearer "+task.Token)
			// MakeRequest(t, downloadReqDenied, http.StatusForbidden)

			// Enable cross-repo access
			require.NoError(t, actions_model.SetOrgActionsConfig(t.Context(), org.ID, &repo_model.ActionsConfig{
				CrossRepoMode: repo_model.ActionsCrossRepoModeAll,
			}))

			// Try to download with cross-repo enabled - should succeed
			downloadReq := NewRequest(t, "GET", packageURL)
			downloadReq.Header.Set("Authorization", "Bearer "+task.Token)
			resp := MakeRequest(t, downloadReq, http.StatusOK)
			assert.Equal(t, content, resp.Body.Bytes(), "Should be able to read package from other repo in same org")

			// Try to upload a package with task token (cross-repo write)
			// Cross-repo access should be read-only, write attempts return 401 Unauthorized
			writePackageURL := fmt.Sprintf("/api/packages/%s/generic/%s/%s/write-test.bin", orgName, packageName, packageVersion)
			writeReq := NewRequestWithBody(t, "PUT", writePackageURL, bytes.NewReader(content))
			writeReq.Header.Set("Authorization", "Bearer "+task.Token)
			MakeRequest(t, writeReq, http.StatusUnauthorized)
		})

		// 7. Test Cross-Repo Access - Specific Repositories
		t.Run("Cross-Repo Access - Specific Repositories", func(t *testing.T) {
			// Set mode to Selected with ONLY repo-B
			require.NoError(t, actions_model.SetOrgActionsConfig(t.Context(), org.ID, &repo_model.ActionsConfig{
				CrossRepoMode:       repo_model.ActionsCrossRepoModeSelected,
				AllowedCrossRepoIDs: []int64{repoBID},
			}))

			// Access to repo-B should succeed
			testCtx.Reponame = "repo-B"
			testCtx.ExpectedCode = http.StatusOK
			doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
				assert.Equal(t, "repo-B", r.Name)
			})(t)

			// Remove repo-B from allowed list
			require.NoError(t, actions_model.SetOrgActionsConfig(t.Context(), org.ID, &repo_model.ActionsConfig{
				CrossRepoMode:       repo_model.ActionsCrossRepoModeSelected,
				AllowedCrossRepoIDs: []int64{}, // Empty list
			}))

			// Access to repo-B should fail (404)
			testCtx.ExpectedCode = http.StatusNotFound
			doAPIGetRepository(testCtx, nil)(t)
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
				permsJSON := repo_model.MarshalTokenPermissions(finalPerms)

				job := &actions_model.ActionRunJob{
					RunID:            run.ID,
					RepoID:           repository.ID,
					OwnerID:          repository.Owner.ID,
					CommitSHA:        "abc123456",
					Name:             jobName,
					JobID:            jobID,
					Status:           actions_model.StatusRunning,
					TokenPermissions: permsJSON,
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
		t.Logf("TokenPermissions: %s", task.Job.TokenPermissions)
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
	_, err := actions_model.UpdateRunJob(t.Context(), job, nil, "task_id")
	require.NoError(t, err)

	task.Job = job
	return task
}
