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
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

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
			require.NoError(t, repo_model.UpdateRepoUnit(t.Context(), actionsUnit))
		}

		require.NoError(t, task.GenerateToken())
		task.Status = actions_model.StatusRunning
		task.IsForkPullRequest = isFork
		err = actions_model.UpdateTask(t.Context(), task, "token_hash", "token_salt", "token_last_eight", "status", "is_fork_pull_request")
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
			task := &actions_model.ActionTask{}
			require.NoError(t, task.GenerateToken())
			task.Status = actions_model.StatusRunning
			task.IsForkPullRequest = false
			task.RepoID = repository.ID
			err := db.Insert(t.Context(), task)
			require.NoError(t, err)

			// Enable Actions unit for the repository
			err = db.Insert(t.Context(), &repo_model.RepoUnit{
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
			actionsCfg.DefaultTokenPermissions = nil // Ensure no custom permissions override the mode
			actionsCfg.MaxTokenPermissions = nil     // Ensure no max permissions interfere
			// Update the config
			actionsUnit.Config = actionsCfg
			require.NoError(t, repo_model.UpdateRepoUnit(t.Context(), actionsUnit))
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
		httpContext := NewAPITestContext(t, "user2", "repo-clamping", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
		t.Run("Create Repository", doAPICreateRepository(httpContext, false, func(t *testing.T, repository structs.Repository) {
			// Enable Actions unit with Clamping Config
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repository.ID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
					MaxTokenPermissions: &repo_model.ActionsTokenPermissions{
						Code: perm.AccessModeRead, // Max is Read - will clamp default Write to Read
					},
				},
			})
			require.NoError(t, err)

			// Create Task and Token
			task := &actions_model.ActionTask{
				RepoID:            repository.ID,
				Status:            actions_model.StatusRunning,
				IsForkPullRequest: false,
			}
			require.NoError(t, task.GenerateToken())
			require.NoError(t, db.Insert(t.Context(), task))

			// Verify Token Permissions
			session := emptyTestSession(t)
			testCtx := APITestContext{
				Session:  session,
				Token:    task.Token,
				Username: "user2",
				Reponame: "repo-clamping",
			}

			// 1. Try to Write (Create File) - Should Fail (403) because Max is Read
			testCtx.ExpectedCode = http.StatusForbidden
			t.Run("Fail to Create File (Max Clamping)", doAPICreateFile(testCtx, "clamping.txt", &structs.CreateFileOptions{
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("test")),
			}))

			// 2. Try to Read (Get Repository) - Should Succeed (200)
			testCtx.ExpectedCode = http.StatusOK
			t.Run("Get Repository (Read Allowed)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
				assert.Equal(t, "repo-clamping", r.Name)
			}))
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
		task := &actions_model.ActionTask{
			RepoID:            repoAID,
			Status:            actions_model.StatusRunning,
			IsForkPullRequest: false,
		}
		require.NoError(t, task.GenerateToken())
		require.NoError(t, db.Insert(t.Context(), task))

		// 5. Verify Access to Repo B (Target)
		testCtx := APITestContext{
			Session:  emptyTestSession(t),
			Token:    task.Token,
			Username: orgName,
			Reponame: "repo-B",
		}

		// Case A: Default (AllowCrossRepoAccess = false/unset) -> Should Fail (404 Not Found)
		// API returns 404 for private repos you can't access, not 403, to avoid leaking existence.
		testCtx.ExpectedCode = http.StatusNotFound
		t.Run("Cross-Repo Access Denied (Default)", doAPIGetRepository(testCtx, nil))

		// Case B: Enable AllowCrossRepoAccess
		org, err := org_model.GetOrgByName(t.Context(), orgName)
		require.NoError(t, err)

		cfg := &repo_model.ActionsConfig{
			AllowCrossRepoAccess: true,
		}
		err = actions_model.SetOrgActionsConfig(t.Context(), org.ID, cfg)
		require.NoError(t, err)

		// Retry -> Should Succeed (200) - Read Only
		testCtx.ExpectedCode = http.StatusOK
		t.Run("Cross-Repo Access Allowed", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
			assert.Equal(t, "repo-B", r.Name)
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
				AllowCrossRepoAccess: false,
			}))

			// Try to download with cross-repo disabled - should fail
			downloadReqDenied := NewRequest(t, "GET", packageURL)
			downloadReqDenied.Header.Set("Authorization", "Bearer "+task.Token)
			MakeRequest(t, downloadReqDenied, http.StatusForbidden)

			// Enable cross-repo access
			require.NoError(t, actions_model.SetOrgActionsConfig(t.Context(), org.ID, &repo_model.ActionsConfig{
				AllowCrossRepoAccess: true,
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
			task := &actions_model.ActionTask{
				RepoID:            repository.ID,
				Status:            actions_model.StatusRunning,
				IsForkPullRequest: false,
			}
			require.NoError(t, task.GenerateToken())
			require.NoError(t, db.Insert(t.Context(), task))

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
			require.NoError(t, repo_model.UpdateRepoUnit(t.Context(), actionsUnit))

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
// This is exactly what the reviewer reported: `permissions: read-all` should restrict write operations.
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

			// Create an Actions run job with TokenPermissions set (simulating a workflow with permissions: read-all)
			// This is what the permission parser does when parsing the workflow YAML
			readOnlyPerms := repo_model.ActionsTokenPermissions{
				Code:         perm.AccessModeRead,
				Issues:       perm.AccessModeRead,
				PullRequests: perm.AccessModeRead,
				Packages:     perm.AccessModeRead,
				Actions:      perm.AccessModeRead,
				Wiki:         perm.AccessModeRead,
			}
			permsJSON := repo_model.MarshalTokenPermissions(readOnlyPerms)

			// Create a run and job with explicit permissions
			run := &actions_model.ActionRun{
				RepoID:    repository.ID,
				OwnerID:   repository.Owner.ID,
				Title:     "Test workflow with read-all permissions",
				Status:    actions_model.StatusRunning,
				Ref:       "refs/heads/master",
				CommitSHA: "abc123",
			}
			require.NoError(t, db.Insert(t.Context(), run))

			job := &actions_model.ActionRunJob{
				RunID:            run.ID,
				RepoID:           repository.ID,
				OwnerID:          repository.Owner.ID,
				CommitSHA:        "abc123",
				Name:             "test-job",
				JobID:            "test-job",
				Status:           actions_model.StatusRunning,
				TokenPermissions: permsJSON, // This is the key - workflow-declared permissions
			}
			require.NoError(t, db.Insert(t.Context(), job))

			// Create task linked to the job
			task := &actions_model.ActionTask{
				JobID:             job.ID,
				RepoID:            repository.ID,
				Status:            actions_model.StatusRunning,
				IsForkPullRequest: false,
			}
			require.NoError(t, task.GenerateToken())
			require.NoError(t, db.Insert(t.Context(), task))

			// Update job with task ID
			job.TaskID = task.ID
			_, err = db.GetEngine(t.Context()).ID(job.ID).Cols("task_id").Update(job)
			require.NoError(t, err)

			// Test: Even though repo is in PERMISSIVE mode, the workflow has permissions: read-all
			// So write operations should FAIL
			session := emptyTestSession(t)
			testCtx := APITestContext{
				Session:  session,
				Token:    task.Token,
				Username: "user2",
				Reponame: "repo-workflow-perms-kw",
			}

			// Read should work
			testCtx.ExpectedCode = http.StatusOK
			t.Run("GITEA_TOKEN Get Repository (Read OK)", doAPIGetRepository(testCtx, func(t *testing.T, r structs.Repository) {
				assert.Equal(t, "repo-workflow-perms-kw", r.Name)
			}))

			// Write should FAIL due to workflow permissions: read-all
			testCtx.ExpectedCode = http.StatusForbidden
			t.Run("GITEA_TOKEN Create File (Write BLOCKED by workflow permissions)", doAPICreateFile(testCtx, "should-fail-due-to-workflow-perms.txt", &structs.CreateFileOptions{
				FileOptions: structs.FileOptions{
					BranchName: "master",
					Message:    "this should fail due to workflow permissions",
				},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("Should Not Be Created")),
			}))

			// Subtest: Verify that job-level overriding works
			// Create another job with `permissions: contents: write` to override `read-all`
			// Logic: Workflow read-all -> Code: Read. Job contents:write -> Code: Write.
			// Repo is Permissive (Max: Write). So Result: Write.
			overridePerms := repo_model.ActionsTokenPermissions{
				Code:         perm.AccessModeWrite,
				Issues:       perm.AccessModeRead,
				PullRequests: perm.AccessModeRead,
				Packages:     perm.AccessModeRead,
				Actions:      perm.AccessModeRead,
				Wiki:         perm.AccessModeRead,
			}
			overridePermsJSON := repo_model.MarshalTokenPermissions(overridePerms)

			jobOverride := &actions_model.ActionRunJob{
				RunID:            run.ID,
				RepoID:           repository.ID,
				OwnerID:          repository.Owner.ID,
				CommitSHA:        "abc123",
				Name:             "test-job-override",
				JobID:            "test-job-override",
				Status:           actions_model.StatusRunning,
				TokenPermissions: overridePermsJSON,
			}
			require.NoError(t, db.Insert(t.Context(), jobOverride))

			taskOverride := &actions_model.ActionTask{
				JobID:             jobOverride.ID,
				RepoID:            repository.ID,
				Status:            actions_model.StatusRunning,
				IsForkPullRequest: false,
			}
			require.NoError(t, taskOverride.GenerateToken())
			require.NoError(t, db.Insert(t.Context(), taskOverride))
			jobOverride.TaskID = taskOverride.ID
			_, err = db.GetEngine(t.Context()).ID(jobOverride.ID).Cols("task_id").Update(jobOverride)
			require.NoError(t, err)

			testCtxOverride := APITestContext{
				Session:  session,
				Token:    taskOverride.Token,
				Username: "user2",
				Reponame: "repo-workflow-perms-kw",
			}
			testCtxOverride.ExpectedCode = http.StatusCreated
			t.Run("GITEA_TOKEN Create File (Write ALLOWED by job override)", doAPICreateFile(testCtxOverride, "should-succeed-override.txt", &structs.CreateFileOptions{
				FileOptions: structs.FileOptions{
					BranchName: "master",
					Message:    "this should succeed due to job permissions override",
				},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("Should Be Created")),
			}))
		}))
	})
}

func TestActionsRerunPermissions(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")
		httpContext := NewAPITestContext(t, "user2", "repo-rerun-perms", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)

		t.Run("Rerun Permissions", doAPICreateRepository(httpContext, false, func(t *testing.T, repository structs.Repository) {
			// 1. Enable Actions with PERMISSIVE mode
			err := db.Insert(t.Context(), &repo_model.RepoUnit{
				RepoID: repository.ID,
				Type:   unit_model.TypeActions,
				Config: &repo_model.ActionsConfig{
					TokenPermissionMode: repo_model.ActionsTokenPermissionModePermissive,
				},
			})
			require.NoError(t, err)

			// 2. Create Run and Job with implicit permissions (no parsed perms stored yet)
			// or with parsed perms that allow write (Permissive default)
			workflowPayload := `
name: Test Rerun
on: workflow_dispatch
jobs:
  test-rerun:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
`
			run := &actions_model.ActionRun{
				RepoID:        repository.ID,
				OwnerID:       repository.Owner.ID,
				Title:         "Test Rerun",
				Status:        actions_model.StatusSuccess, // Run finished
				Ref:           "refs/heads/master",
				CommitSHA:     "abc123",
				WorkflowID:    "test-rerun.yaml",
				TriggerUserID: repository.Owner.ID,
			}
			require.NoError(t, db.Insert(t.Context(), run))

			// Initial permissions: Permissive (Write)
			initialPerms := repo_model.ActionsTokenPermissions{
				Code: perm.AccessModeWrite,
			}

			job := &actions_model.ActionRunJob{
				RunID:            run.ID,
				RepoID:           repository.ID,
				OwnerID:          repository.Owner.ID,
				CommitSHA:        "abc123",
				Name:             "test-rerun",
				JobID:            "test-rerun",
				Status:           actions_model.StatusSuccess, // Job finished
				WorkflowPayload:  []byte(workflowPayload),
				TokenPermissions: repo_model.MarshalTokenPermissions(initialPerms),
			}
			require.NoError(t, db.Insert(t.Context(), job))

			// 3. Change Repo Settings to RESTRICTED
			// We need to update the RepoUnit config
			unitConfig := &repo_model.ActionsConfig{
				TokenPermissionMode: repo_model.ActionsTokenPermissionModeRestricted,
			}
			// Update the specific unit
			// Need to find the unit first
			repo, err := repo_model.GetRepositoryByID(t.Context(), repository.ID)
			require.NoError(t, err)
			unit, err := repo.GetUnit(t.Context(), unit_model.TypeActions)
			require.NoError(t, err)

			unit.Config = unitConfig
			require.NoError(t, repo_model.UpdateRepoUnit(t.Context(), unit))

			// 4. Trigger Rerun via Web Handler
			// POST /:username/:reponame/actions/runs/:index/rerun
			// We need to know operation run index. Since it's the first run, it should be 1?
			// ActionRun.Index is auto-increment but not set in my insert.
			// Ideally we use CreateRun which handles index.
			// Let's manually set index 1.
			run.Index = 1
			_, err = db.GetEngine(t.Context()).ID(run.ID).Cols("index").Update(run)
			require.NoError(t, err)

			req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", "user2", "repo-rerun-perms", run.Index))
			session.MakeRequest(t, req, http.StatusOK)

			// 5. Verify TokenPermissions in DB are now Restricted (Read-only)
			// Reload job
			jobReload := new(actions_model.ActionRunJob)
			has, err := db.GetEngine(t.Context()).ID(job.ID).Get(jobReload)
			require.NoError(t, err)
			assert.True(t, has)

			// Check permissions
			perms, err := repo_model.UnmarshalTokenPermissions(jobReload.TokenPermissions)
			require.NoError(t, err)

			// Should be restricted (Read)
			assert.Equal(t, perm.AccessModeRead, perms.Code, "Permissions should be restricted to Read after rerun in restricted mode")
		}))
	})
}
