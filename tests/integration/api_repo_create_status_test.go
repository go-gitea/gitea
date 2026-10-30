// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPICreateRepoWithExistingFilesStatus(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeAll)

	// A repository directory that exists on disk without a database row is what
	// a failed delete leaves behind. Creating a repository with the same name
	// must answer 409, as the migrate endpoint does, not 500.
	name := "repo-files-already-exist"
	repoPath := filepath.Join(setting.RepoRootPath, user.LowerName, name+".git")
	assert.NoError(t, os.MkdirAll(repoPath, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(repoPath) })

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{Name: name}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusConflict)
}
