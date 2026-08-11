// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func TestDeployTokenGitHTTP(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	readKey, err := asymkey_model.AddDeployToken(t.Context(), repo.ID, "read", true)
	require.NoError(t, err)
	writeKey, err := asymkey_model.AddDeployToken(t.Context(), repo.ID, "write", false)
	require.NoError(t, err)

	requestAs := func(t *testing.T, token, path string, expected int) {
		MakeRequest(t, NewRequest(t, "GET", path).AddBasicAuth("deploy-token", token), expected)
	}

	t.Run("Clone", func(t *testing.T) {
		requestAs(t, readKey.Token, "/"+repo.FullName()+"/info/refs?service=git-upload-pack", http.StatusOK)
	})
	t.Run("PushWithReadToken", func(t *testing.T) {
		requestAs(t, readKey.Token, "/"+repo.FullName()+"/info/refs?service=git-receive-pack", http.StatusForbidden)
	})
	t.Run("PushWithWriteToken", func(t *testing.T) {
		requestAs(t, writeKey.Token, "/"+repo.FullName()+"/info/refs?service=git-receive-pack", http.StatusOK)
	})
	t.Run("OtherRepo", func(t *testing.T) {
		requestAs(t, readKey.Token, "/"+otherRepo.FullName()+"/info/refs?service=git-upload-pack", http.StatusNotFound)
	})
	t.Run("UnknownToken", func(t *testing.T) {
		requestAs(t, asymkey_model.DeployTokenPrefix+"0123456789abcdef", "/"+repo.FullName()+"/info/refs?service=git-upload-pack", http.StatusUnauthorized)
	})
	t.Run("RejectedOutsideGitHTTP", func(t *testing.T) {
		// the owner of the repo would be able to read it, the token must not act as that owner
		requestAs(t, readKey.Token, "/api/v1/repos/"+repo.FullName(), http.StatusUnauthorized)
	})
}
