// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageSettingsLinkRequiresRepoPermission(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})
	newRepo := func(name string, isPrivate bool) *repo_model.Repository {
		repo := &repo_model.Repository{OwnerID: org.ID, OwnerName: org.Name, Name: name, LowerName: name, IsPrivate: isPrivate}
		require.NoError(t, db.Insert(t.Context(), repo))
		require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePackages}))
		return repo
	}
	teamRepo, otherRepo := newRepo("team-repo", false), newRepo("other-repo", true)
	pkg := &packages_model.Package{OwnerID: org.ID, Type: packages_model.TypeGeneric, Name: "package", LowerName: "package"}
	require.NoError(t, db.Insert(t.Context(), pkg))

	postLink := func(doerID int64, repoName string) int {
		ctx, resp := contexttest.MockContext(t, "privated_org/-/packages/settings/generic/package")
		contexttest.LoadUser(t, ctx, doerID)
		ctx.Package = &context.Package{Descriptor: &packages_model.PackageDescriptor{Package: unittest.AssertExistsAndLoadBean(t, &packages_model.Package{ID: pkg.ID}), Owner: org}}
		packageSettingsPostActionLink(ctx, &forms.PackageSettingForm{RepoName: repoName})
		return resp.Code
	}
	linkedRepoID := func() int64 {
		return unittest.AssertExistsAndLoadBean(t, &packages_model.Package{ID: pkg.ID}).RepoID
	}

	assert.Equal(t, http.StatusForbidden, postLink(5, teamRepo.Name))
	assert.Zero(t, linkedRepoID())

	require.NoError(t, organization.AddTeamRepo(t.Context(), org.ID, 17, teamRepo.ID))
	assert.Equal(t, http.StatusOK, postLink(5, teamRepo.Name))
	assert.Equal(t, teamRepo.ID, linkedRepoID())

	assert.Equal(t, http.StatusForbidden, postLink(5, ""))
	assert.Equal(t, teamRepo.ID, linkedRepoID())

	assert.Equal(t, http.StatusOK, postLink(1, otherRepo.Name))
	assert.Equal(t, otherRepo.ID, linkedRepoID())

	assert.Equal(t, http.StatusForbidden, postLink(5, teamRepo.Name))
	assert.Equal(t, otherRepo.ID, linkedRepoID())

	require.NoError(t, db.Insert(t.Context(), &access_model.Access{UserID: 5, RepoID: otherRepo.ID, Mode: perm.AccessModeAdmin}))
	assert.Equal(t, http.StatusOK, postLink(5, teamRepo.Name))
	assert.Equal(t, teamRepo.ID, linkedRepoID())
}
