// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	repo_service "gitea.dev/services/repository"

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
		require.NoError(t, repo_model.WatchRepoAuto(t.Context(), watcher, repo, true))
		require.NoError(t, repo_model.StarRepo(t.Context(), watcher, repo, true))
		unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})

		require.NoError(t, ChangeOrganizationVisibility(t.Context(), org, structs.VisibleTypePrivate))

		// making the org private must drop watches, not only stars, from users who can no longer see it
		unittest.AssertNotExistsBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})
		unittest.AssertNotExistsBean(t, &repo_model.Star{UID: watcher.ID, RepoID: repo.ID})
	})
}

func TestSetOrganizationArchived(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	_, err := db.GetEngine(ctx).ID(32).Cols("is_archived", "archived_unix").
		Update(&repo_model.Repository{IsArchived: true, ArchivedUnix: 1})
	require.NoError(t, err)

	err = SetOrganizationArchived(ctx, org, true)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
	errHasMirrors, ok := errors.AsType[ErrOrgHasMirrors](err)
	require.True(t, ok)
	assert.Equal(t, []string{"repo5"}, errHasMirrors.Mirrors)
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).IsArchived)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3, IsArchived: false})

	_, err = db.GetEngine(ctx).ID(5).Cols("is_mirror").Update(&repo_model.Repository{IsMirror: false})
	require.NoError(t, err)
	require.NoError(t, SetOrganizationArchived(ctx, org, true))
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).IsArchived)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3, IsArchived: true})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5, IsArchived: true})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32, IsArchived: true, ArchivedUnix: 1})

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	_, err = repo_service.CreateRepositoryDirectly(ctx, doer, org.AsUser(), repo_service.CreateRepoOptions{Name: "new-repo"}, false)
	assert.ErrorIs(t, err, util.ErrPermissionDenied)

	require.NoError(t, SetOrganizationArchived(ctx, org, false))
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).IsArchived)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3, IsArchived: true})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32, IsArchived: true, ArchivedUnix: 1})
}
