// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
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
		require.NoError(t, task.GenerateToken())
		task.Status = actions_model.StatusRunning
		task.IsForkPullRequest = isFork
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
			actionsUnit := repo.MustGetUnit(t.Context(), unit_model.TypeActions)
			actionsCfg := actionsUnit.ActionsConfig()
			actionsCfg.TokenPermissionMode = repo_model.ActionsTokenPermissionMode(mode)
			actionsCfg.DefaultTokenPermissions = nil // Ensure no custom permissions override the mode
			actionsCfg.MaxTokenPermissions = nil     // Ensure no max permissions interfere
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
				context.Session.MakeRequest(t, req, http.StatusNoContent)
			})
		}
	}
}
