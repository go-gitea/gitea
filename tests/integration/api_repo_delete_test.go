// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepositoryDelete(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("AdminNoPermToDeleteRepo", func(t *testing.T) {
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
		teams, err := organization.GetUserOrgTeams(t.Context(), org.ID, doer.ID)
		require.NoError(t, err)
		require.Len(t, teams, 1)

		team := teams[0]
		assert.Equal(t, perm.AccessModeAdmin, team.AccessMode)
		assert.True(t, team.CanCreateOrgRepo)

		targetRepo, err := repo_service.CreateRepository(t.Context(), owner, org, repo_service.CreateRepoOptions{Name: "target-admin-team"})
		require.NoError(t, err)
		require.NoError(t, repo_service.TeamAddRepository(t.Context(), team, targetRepo))

		token := getUserToken(t, doer.Name, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequest(t, "DELETE", "/api/v1/repos/"+targetRepo.FullName()).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: targetRepo.ID})
	})
}
