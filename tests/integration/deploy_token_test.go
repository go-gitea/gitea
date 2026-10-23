// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	deploykey_model "codenest.dev/models/deploykey"
	"codenest.dev/models/perm"
	repo_model "codenest.dev/models/repo"
	"codenest.dev/models/unittest"
	"codenest.dev/modules/git/gitcmd"
	lfs_module "codenest.dev/modules/lfs"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployTokenGitHTTP(t *testing.T) {
	onCodeNestRun(t, func(t *testing.T, u *url.URL) {
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
			pushURL := *u
			pushURL.Path = "/" + repo.FullName() + ".git"
			pushURL.User = url.UserPassword("deploy-token", readKey.Token)
			_, _, err := gitcmd.NewCommand("push").AddDynamicArguments(pushURL.String(), "HEAD:refs/heads/read-token-push").WithRepo(repo).RunStdString(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Stderr(), "User permission denied for writing.")
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
	})
}
