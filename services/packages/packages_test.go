// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestUnlinkFromRepositoryRequiresTargetRepoAdmin(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := &repo_model.Repository{OwnerID: 3, OwnerName: "org3", Name: "package-repo", LowerName: "package-repo", IsPrivate: true}
	require.NoError(t, db.Insert(t.Context(), repo))
	require.NoError(t, db.Insert(t.Context(),
		&repo_model.RepoUnit{RepoID: repo.ID, Type: unit_model.TypeCode},
		&repo_model.RepoUnit{RepoID: repo.ID, Type: unit_model.TypePackages},
		&organization.TeamRepo{OrgID: repo.OwnerID, TeamID: 14, RepoID: repo.ID},
	))
	pkg := &packages_model.Package{OwnerID: repo.OwnerID, RepoID: repo.ID, Type: packages_model.TypeGeneric, Name: "package", LowerName: "package"}
	require.NoError(t, db.Insert(t.Context(), pkg))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})

	assert.Error(t, UnlinkFromRepository(t.Context(), pkg, doer))
	assert.Equal(t, repo.ID, unittest.AssertExistsAndLoadBean(t, &packages_model.Package{ID: pkg.ID}).RepoID)

	require.NoError(t, db.Insert(t.Context(), &organization.TeamRepo{OrgID: repo.OwnerID, TeamID: 12, RepoID: repo.ID}))
	require.NoError(t, UnlinkFromRepository(t.Context(), pkg, doer))
	assert.Zero(t, unittest.AssertExistsAndLoadBean(t, &packages_model.Package{ID: pkg.ID}).RepoID)
}
