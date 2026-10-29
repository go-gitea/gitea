// Copyright 2022 The Gitea Authors. All rights reserved.
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
	api "gitea.dev/modules/structs"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoCollaboratorPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	repo2Owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo2.OwnerID})

	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	user10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	user11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})
	user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})

	testCtx := NewAPITestContext(t, repo2Owner.Name, repo2.Name, auth_model.AccessTokenScopeWriteRepository)

	t.Run("RepoOwnerShouldBeOwner", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission", repo2Owner.Name, repo2.Name, repo2Owner.Name).
			AddTokenAuth(testCtx.Token)
		resp := MakeRequest(t, req, http.StatusOK)

		repoPermission := DecodeJSON(t, resp, &api.RepoCollaboratorPermission{})

		assert.Equal(t, api.AccessLevelNameOwner, repoPermission.Permission)
	})

	for _, testCase := range []struct {
		name       string
		mode       perm.AccessMode
		permission api.AccessLevelName
	}{
		{"Read", perm.AccessModeRead, api.AccessLevelNameRead},
		{"Write", perm.AccessModeWrite, api.AccessLevelNameWrite},
		{"Admin", perm.AccessModeAdmin, api.AccessLevelNameAdmin},
	} {
		t.Run("CollaboratorWith"+testCase.name+"Access", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWith"+testCase.name+"Access", doAPIAddCollaborator(testCtx, user4.Name, testCase.mode))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission", repo2Owner.Name, repo2.Name, user4.Name).
				AddTokenAuth(testCtx.Token)
			resp := MakeRequest(t, req, http.StatusOK)

			repoPermission := DecodeJSON(t, resp, &api.RepoCollaboratorPermission{})

			assert.Equal(t, testCase.permission, repoPermission.Permission)
		})
	}

	t.Run("CollaboratorNotFound", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission", repo2Owner.Name, repo2.Name, "non-existent-user").
			AddTokenAuth(testCtx.Token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("CollaboratorBlocked", func(t *testing.T) {
		ctx := NewAPITestContext(t, repo2Owner.Name, repo2.Name, auth_model.AccessTokenScopeWriteRepository)
		ctx.ExpectedCode = http.StatusForbidden
		doAPIAddCollaborator(ctx, user34.Name, perm.AccessModeAdmin)(t)
	})

	for _, readOwnPermission := range []bool{true, false} {
		t.Run("CollaboratorCanQueryItsPermissions", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user5.Name, perm.AccessModeRead))

			_session := loginUser(t, user5.Name)
			_testCtx := NewAPITestContext(t, user5.Name, repo2.Name, auth_model.AccessTokenScopeReadRepository)

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission", repo2Owner.Name, repo2.Name, user5.Name).
				AddTokenAuth(_testCtx.Token)
			resp := _session.MakeRequest(t, req, http.StatusOK)

			repoPermission := DecodeJSON(t, resp, &api.RepoCollaboratorPermission{})

			assert.Equal(t, api.AccessLevelNameRead, repoPermission.Permission)

			if readOwnPermission {
				t.Run("CollaboratorCanReadOwnPermission", func(t *testing.T) {
					session := loginUser(t, user5.Name)
					token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

					req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission", repo2Owner.Name, repo2.Name, user5.Name).AddTokenAuth(token)
					resp = MakeRequest(t, req, http.StatusOK)

					repoCollPerm := DecodeJSON(t, resp, &api.RepoCollaboratorPermission{})

					assert.Equal(t, api.AccessLevelNameRead, repoCollPerm.Permission)
				})
			}
		})
	}

	t.Run("RepoAdminCanQueryACollaboratorsPermissions", func(t *testing.T) {
		t.Run("AddUserAsCollaboratorWithAdminAccess", doAPIAddCollaborator(testCtx, user10.Name, perm.AccessModeAdmin))
		t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user11.Name, perm.AccessModeRead))

		_session := loginUser(t, user10.Name)
		_testCtx := NewAPITestContext(t, user10.Name, repo2.Name, auth_model.AccessTokenScopeReadRepository)

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission", repo2Owner.Name, repo2.Name, user11.Name).
			AddTokenAuth(_testCtx.Token)
		resp := _session.MakeRequest(t, req, http.StatusOK)

		repoPermission := DecodeJSON(t, resp, &api.RepoCollaboratorPermission{})

		assert.Equal(t, api.AccessLevelNameRead, repoPermission.Permission)
	})
}

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
		token := getUserToken(t, doer.Name, auth_model.AccessTokenScopeWriteRepository)

		unrelatedRepo, err := repo_service.CreateRepository(t.Context(), owner, org, repo_service.CreateRepoOptions{Name: "unrelated-admin-team"})
		require.NoError(t, err)

		targetRepo, err := repo_service.CreateRepository(t.Context(), owner, org, repo_service.CreateRepoOptions{Name: "target-admin-team"})
		require.NoError(t, err)
		require.NoError(t, repo_service.TeamAddRepository(t.Context(), team, targetRepo))

		req := NewRequest(t, "DELETE", "/api/v1/repos/"+unrelatedRepo.FullName()).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: unrelatedRepo.ID})

		req = NewRequest(t, "DELETE", "/api/v1/repos/"+targetRepo.FullName()).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		unittest.AssertNotExistsBean(t, &repo_model.Repository{ID: targetRepo.ID})
	})
}
