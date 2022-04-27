// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoCollaboratorPermission(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		//user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
		user20 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 20}).(*user_model.User)
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
		repo1Owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo1.OwnerID}).(*user_model.User)

		// Login as User2.
		session := loginUser(t, repo1Owner.Name)
		testCtx := NewAPITestContext(t, repo1Owner.Name, repo1.Name)

		t.Run("RepoOwnerShouldBeAdmin", func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, repo1Owner.Name, testCtx.Token)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "owner", repoPermission.Permission)
		})

		t.Run("CollaboratorWithReadAccess", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithReadAccess", doAPIAddCollaborator(testCtx, user4.Name, perm.AccessModeRead))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, user4.Name, testCtx.Token)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})

		t.Run("CollaboratorWithWriteAccess", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithWriteAccess", doAPIAddCollaborator(testCtx, user4.Name, perm.AccessModeWrite))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, user4.Name, testCtx.Token)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "write", repoPermission.Permission)
		})

		t.Run("CollaboratorWithAdminAccess", func(t *testing.T) {
			t.Run("AddUserAsCollaboratorWithAdminAccess", doAPIAddCollaborator(testCtx, user4.Name, perm.AccessModeAdmin))

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, user4.Name, testCtx.Token)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "admin", repoPermission.Permission)
		})

		t.Run("WhyHasUser20ReadAccess", func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, user20.Name, testCtx.Token)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})

		t.Run("WhyHasUser5ReadAccess", func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, "user5", testCtx.Token)
			resp := session.MakeRequest(t, req, http.StatusOK)

			var repoPermission api.RepoCollaboratorPermission
			DecodeJSON(t, resp, &repoPermission)

			assert.Equal(t, "read", repoPermission.Permission)
		})

		t.Run("CollaboratorNotFound", func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/collaborators/%s/permission?token=%s", repo1Owner.Name, repo1.Name, "non-existent-user", testCtx.Token)
			session.MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
