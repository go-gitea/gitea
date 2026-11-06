// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	"gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"
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

	t.Run("PurgeClearsPackagesThatWouldOtherwiseBlockIt", func(t *testing.T) {
		// Packages block deletion exactly as repositories do, but a purge
		// cleared only the repositories -- and the web UI gives no way to
		// remove a container package's untagged manifest versions in bulk, so
		// an organization that had pushed an image could not be deleted by
		// any sequence of actions available to its owner.
		defer test.MockVariableValue(&setting.Packages.Enabled, true)()

		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 7})
		pkg, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
			OwnerID:   org.ID,
			Type:      packages_model.TypeContainer,
			Name:      "retained",
			LowerName: "retained",
		})
		require.NoError(t, err)
		version, err := packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
			PackageID:    pkg.ID,
			LowerVersion: "1.0",
			Version:      "1.0",
			MetadataJSON: "{}",
		})
		require.NoError(t, err)

		owns, err := packages_model.HasOwnerPackages(t.Context(), org.ID)
		require.NoError(t, err)
		require.True(t, owns, "the fixture does not reproduce what blocked the purge")

		require.NoError(t, DeleteOrganization(t.Context(), org, true))
		unittest.AssertNotExistsBean(t, &organization.Organization{ID: 7})
		// The versions are what the ownership check counts, and what the UI
		// could not clear. The package row itself outlives them until the
		// cleanup job runs, exactly as it does when a user is deleted.
		unittest.AssertNotExistsBean(t, &packages_model.PackageVersion{ID: version.ID})
		owns, err = packages_model.HasOwnerPackages(t.Context(), org.ID)
		require.NoError(t, err)
		assert.False(t, owns)
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
