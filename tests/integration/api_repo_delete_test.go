// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func TestAPIDeleteRepositoryRequiresTargetRepoAdmin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 12})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	unrelatedRepo, err := repo_service.CreateRepository(t.Context(), owner, org, repo_service.CreateRepoOptions{Name: "unrelated-admin-team"})
	require.NoError(t, err)
	defer func() {
		_ = repo_service.DeleteRepositoryDirectly(t.Context(), unrelatedRepo.ID)
	}()
	targetRepo, err := repo_service.CreateRepository(t.Context(), owner, org, repo_service.CreateRepoOptions{Name: "target-admin-team"})
	require.NoError(t, err)
	require.NoError(t, repo_service.TeamAddRepository(t.Context(), team, targetRepo))

	token := getUserToken(t, doer.Name, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequest(t, "DELETE", "/api/v1/repos/"+unrelatedRepo.FullName()).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: unrelatedRepo.ID})

	req = NewRequest(t, "DELETE", "/api/v1/repos/"+targetRepo.FullName()).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &repo_model.Repository{ID: targetRepo.ID})
}
