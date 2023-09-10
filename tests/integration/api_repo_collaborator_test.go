// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoCollaboratorPermission(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		repo2Owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo2.OwnerID})

		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		user10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
		user11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})

		testCtx := NewAPITestContext(t, repo2Owner.Name, repo2.Name, auth_model.AccessTokenScopeWriteRepository)

		t.Run("RepoOwnerShouldBeOwner", func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, repo2Owner.Name, testCtx.Token)
			resp := MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "owner", repoPermission.Permission)
		})

		t.Run("CollaboratorWithReadAccess", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user4.Name, perm.AccessModeRead))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, user4.Name, testCtx.Token)
			resp := MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})

		t.Run("CollaboratorWithWriteAccess", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithWriteAccess", doAPIAddCollaborator(testCtx, user4.Name, perm.AccessModeWrite))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, user4.Name, testCtx.Token)
			resp := MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "write", repoPermission.Permission)
		})

		t.Run("CollaboratorWithAdminAccess", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithAdminAccess", doAPIAddCollaborator(testCtx, user4.Name, perm.AccessModeAdmin))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, user4.Name, testCtx.Token)
			resp := MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "admin", repoPermission.Permission)
		})

		t.Run("CollaboratorNotFound", func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, "non-existent-user", testCtx.Token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("CollaboratorCanQueryItsPermissions", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user5.Name, perm.AccessModeRead))

			_session := loginUser(t, user5.Name)
			_testCtx := NewAPITestContext(t, user5.Name, repo2.Name, auth_model.AccessTokenScopeReadRepository)

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, user5.Name, _testCtx.Token)
			resp := _session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})

		t.Run("CollaboratorCanQueryItsPermissions", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user5.Name, perm.AccessModeRead))

			_session := loginUser(t, user5.Name)
			_testCtx := NewAPITestContext(t, user5.Name, repo2.Name, auth_model.AccessTokenScopeReadRepository)

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, user5.Name, _testCtx.Token)
			resp := _session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})

		t.Run("RepoAdminCanQueryACollaboratorsPermissions", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithAdminAccess", doAPIAddCollaborator(testCtx, user10.Name, perm.AccessModeAdmin))
			t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user11.Name, perm.AccessModeRead))

			_session := loginUser(t, user10.Name)
			_testCtx := NewAPITestContext(t, user10.Name, repo2.Name, auth_model.AccessTokenScopeReadRepository)

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo2Owner.Name, repo2.Name, user11.Name, _testCtx.Token)
			resp := _session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})
	})
}
