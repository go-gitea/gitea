// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	deploykey_model "gitea.dev/models/deploykey"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	lfs_module "gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtectedBranchDeletionByDeployToken(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeWriteRepository)
		writeKey, err := deploykey_model.AddDeployKeyToken(t.Context(), repo.ID, "delete-write", perm.AccessModeWrite)
		require.NoError(t, err)
		readKey, err := deploykey_model.AddDeployKeyToken(t.Context(), repo.ID, "delete-read", perm.AccessModeRead)
		require.NoError(t, err)
		u.Path = ctx.GitPath()
		u.User = url.UserPassword("deploy-token", writeKey.Token)
		dstPath := t.TempDir()
		require.NoError(t, git.Clone(t.Context(), u.String(), dstPath, git.CloneRepoOptions{}))

		for _, tt := range []struct {
			name              string
			unprotected       bool
			enablePush        bool
			pushAllowlist     bool
			allowDeployPush   bool
			enableDeletion    bool
			deletionAllowlist bool
			allowDeployDelete bool
			readOnly          bool
			defaultBranch     bool
			wantError         string
		}{
			{name: "Unprotected", unprotected: true},
			{name: "DeletionDisabled", enablePush: true, wantError: "protected from deletion"},
			{name: "UnrestrictedDeletion", enablePush: true, enableDeletion: true},
			{name: "NotAllowlisted", enablePush: true, enableDeletion: true, deletionAllowlist: true, wantError: "protected from deletion"},
			{name: "Allowlisted", enablePush: true, pushAllowlist: true, allowDeployPush: true, enableDeletion: true, deletionAllowlist: true, allowDeployDelete: true},
			{name: "NoPushPermission", enablePush: true, pushAllowlist: true, enableDeletion: true, deletionAllowlist: true, allowDeployDelete: true, wantError: "protected from deletion"},
			{name: "PushDisabled", enableDeletion: true, deletionAllowlist: true, allowDeployDelete: true, wantError: "protected from deletion"},
			{name: "ReadOnly", enablePush: true, pushAllowlist: true, allowDeployPush: true, enableDeletion: true, deletionAllowlist: true, allowDeployDelete: true, readOnly: true},
			{name: "DefaultBranch", enablePush: true, enableDeletion: true, defaultBranch: true, wantError: "is the default branch and cannot be deleted"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				branch := "deploy-token-delete-" + tt.name
				if tt.defaultBranch {
					branch = repo.DefaultBranch
				} else {
					_, _, err := gitcmd.NewCommand("push").AddDynamicArguments(u.String(), "HEAD:refs/heads/"+branch).
						WithDir(dstPath).RunStdString(t.Context())
					require.NoError(t, err)
				}
				if !tt.unprotected {
					req := NewRequestWithJSON(t, http.MethodPost, "/api/v1/repos/"+repo.FullName()+"/branch_protections", &api.CreateBranchProtectionOption{
						RuleName:                    branch,
						EnablePush:                  tt.enablePush,
						EnablePushWhitelist:         tt.pushAllowlist,
						PushWhitelistDeployKeys:     tt.allowDeployPush,
						EnableDeletion:              tt.enableDeletion,
						EnableDeletionAllowlist:     tt.deletionAllowlist,
						DeletionAllowlistDeployKeys: tt.allowDeployDelete,
					}).AddTokenAuth(ctx.Token)
					ctx.Session.MakeRequest(t, req, http.StatusCreated)
				}

				deleteURL := *u
				if tt.readOnly {
					deleteURL.User = url.UserPassword("deploy-token", readKey.Token)
				}
				_, stderr, err := gitcmd.NewCommand("push", "--delete").AddDynamicArguments(deleteURL.String(), branch).
					WithDir(dstPath).RunStdString(t.Context())
				if tt.wantError != "" || tt.readOnly {
					require.Error(t, err)
					if tt.wantError != "" {
						assert.Contains(t, stderr, tt.wantError)
					}
					assert.True(t, git.IsBranchExist(t.Context(), repo, branch))
				} else {
					require.NoError(t, err, "%s", stderr)
					assert.False(t, git.IsBranchExist(t.Context(), repo, branch))
				}
			})
		}
	})
}

func TestDeployTokenGitHTTP(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// need to disable agit, otherwise the "write" permission check is skipped at pre-receive (git-receive-pack) step
	defer test.MockVariableValue(&git.DefaultFeatures().SupportProcReceive, false)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	readKey, err := deploykey_model.AddDeployKeyToken(t.Context(), repo.ID, "read", perm.AccessModeRead)
	require.NoError(t, err)
	writeKey, err := deploykey_model.AddDeployKeyToken(t.Context(), repo.ID, "write", perm.AccessModeWrite)
	require.NoError(t, err)

	requestAs := func(t *testing.T, token, path string, expected int) {
		MakeRequest(t, NewRequest(t, "GET", path).AddBasicAuth("deploy-token", token), expected)
	}

	t.Run("Clone", func(t *testing.T) {
		requestAs(t, readKey.Token, "/"+repo.FullName()+"/info/refs?service=git-upload-pack", http.StatusOK)
	})
	t.Run("PushWithReadToken", func(t *testing.T) {
		requestAs(t, readKey.Token, "/"+repo.FullName()+"/info/refs?service=git-receive-pack", http.StatusNotFound)
	})
	t.Run("PushWithWriteToken", func(t *testing.T) {
		requestAs(t, writeKey.Token, "/"+repo.FullName()+"/info/refs?service=git-receive-pack", http.StatusOK)
	})
	t.Run("OtherRepo", func(t *testing.T) {
		requestAs(t, readKey.Token, "/"+otherRepo.FullName()+"/info/refs?service=git-upload-pack", http.StatusNotFound)
	})
	t.Run("UnknownToken", func(t *testing.T) {
		requestAs(t, deploykey_model.DeployTokenPrefix+"0123456789abcdef", "/"+repo.FullName()+"/info/refs?service=git-upload-pack", http.StatusUnauthorized)
	})
	t.Run("RejectedOutsideGitHTTP", func(t *testing.T) {
		// the owner of the repo would be able to read it, the token must not act as that owner
		requestAs(t, readKey.Token, "/api/v1/repos/"+repo.FullName(), http.StatusUnauthorized)
	})

	t.Run("LFS", func(t *testing.T) {
		defer test.MockVariableValue(&setting.LFS.StartServer, true)()

		batchAs := func(t *testing.T, token, repoName, operation string, expected int) {
			req := NewRequestWithJSON(t, "POST", "/"+repoName+"/info/lfs/objects/batch", lfs_module.BatchRequest{Operation: operation}).
				AddBasicAuth("deploy-token", token).
				SetHeader("Accept", lfs_module.AcceptHeader).
				SetHeader("Content-Type", lfs_module.MediaType)
			MakeRequest(t, req, expected)
		}

		batchAs(t, readKey.Token, repo.FullName(), "download", http.StatusOK)
		batchAs(t, readKey.Token, repo.FullName(), "upload", http.StatusUnauthorized)
		batchAs(t, writeKey.Token, repo.FullName(), "upload", http.StatusOK)
		batchAs(t, readKey.Token, otherRepo.FullName(), "download", http.StatusUnauthorized)
	})
}
