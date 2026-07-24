// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestOrg(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("UpdateOrgEmailAddress", func(t *testing.T) {
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		originalEmail := org.Email

		require.NoError(t, UpdateOrgEmailAddress(t.Context(), org, nil))
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: originalEmail})

		newEmail := "contact@org3.example.com"
		require.NoError(t, UpdateOrgEmailAddress(t.Context(), org, &newEmail))
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: newEmail})

		invalidEmail := "invalid email"
		err := UpdateOrgEmailAddress(t.Context(), org, &invalidEmail)
		require.ErrorIs(t, err, util.ErrInvalidArgument)
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: newEmail})

		require.NoError(t, UpdateOrgEmailAddress(t.Context(), org, new("")))
		org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: ""})
		assert.Empty(t, org.Email)
	})

	t.Run("DeleteOrganization", func(t *testing.T) {
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})
		assert.NoError(t, DeleteOrganization(t.Context(), org, false))
		unittest.AssertNotExistsBean(t, &organization.Organization{ID: 6})
		unittest.AssertNotExistsBean(t, &organization.OrgUser{OrgID: 6})
		unittest.AssertNotExistsBean(t, &organization.Team{OrgID: 6})

		org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		err := DeleteOrganization(t.Context(), org, false)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrUserOwnRepos(err))

		user := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 5})
		assert.Error(t, DeleteOrganization(t.Context(), user, false))
		unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
	})

	t.Run("ChangeVisibilityWithUserFork", func(t *testing.T) {
		// org 19 has a repository 27 which has a forked repository 29 by user 20
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 19})
		require.NoError(t, ChangeOrganizationVisibility(t.Context(), org, structs.VisibleTypePrivate))
		unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: org.ID, Visibility: structs.VisibleTypePrivate})
	})

	t.Run("ChangeVisibilityClearsWatchesAndStars", func(t *testing.T) {
		// org3 is a public organization owning the public repo32
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		require.Equal(t, structs.VisibleTypePublic, org.Visibility)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32, OwnerID: org.ID})

		// an outside user watches and stars the repo while the org is still visible
		watcher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		require.NoError(t, repo_model.WatchRepo(t.Context(), watcher, repo, true))
		require.NoError(t, repo_model.StarRepo(t.Context(), watcher, repo, true))
		unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})

		require.NoError(t, ChangeOrganizationVisibility(t.Context(), org, structs.VisibleTypePrivate))

		// making the org private must drop watches, not only stars, from users who can no longer see it
		unittest.AssertNotExistsBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})
		unittest.AssertNotExistsBean(t, &repo_model.Star{UID: watcher.ID, RepoID: repo.ID})
	})
}

func TestDeleteOrganizationCleansCodespaceOwnerResources(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})
	manager := insertOrgTestCodespaceManager(t, org.ID)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerToken{
		OwnerID: org.ID,
		Token:   "org-delete-codespace-token",
	}))
	codespaceUUID := "72727272-7272-4272-8272-727272727272"
	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:              codespaceUUID,
		UserID:            1,
		RepoID:            0,
		ManagerID:         manager.ID,
		RefType:           "branch",
		RefName:           "main",
		RepoTag:           "default",
		GitProtocol:       codespace_model.GitProtocolHTTP,
		CommitSHA:         "0123456789abcdef0123456789abcdef01234567",
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 72,
		AutoStopMode:      codespace_model.AutoStopModeDefault,
		CreatedUnix:       1,
		UpdatedUnix:       1,
		LogFilename:       codespaceUUID + ".log",
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceUUID:  codespaceUUID,
		TokenHash:      "org-delete-hash",
		TokenSalt:      "salt",
		TokenLastEight: "last0072",
		TokenEncrypted: "encrypted",
	}))

	require.NoError(t, DeleteOrganization(t.Context(), org, false))

	unittest.AssertNotExistsBean(t, &organization.Organization{ID: org.ID})
	assertOrgTestNotExists(t, new(codespace_model.Manager), "id = ?", manager.ID)
	assertOrgTestNotExists(t, new(codespace_model.ManagerToken), "owner_id = ?", org.ID)
	assertOrgTestNotExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
	assertOrgTestNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
}

func insertOrgTestCodespaceManager(t *testing.T, ownerID int64) *codespace_model.Manager {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "org-delete-manager",
		OwnerID:        ownerID,
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       "[]",
		CreatedUnix:    1,
		LastOnlineUnix: 1,
		MetaJSON:       "{}",
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager
}

func assertOrgTestNotExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.False(t, has)
}
