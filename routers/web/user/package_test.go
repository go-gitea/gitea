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
	repo := &repo_model.Repository{OwnerID: org.ID, OwnerName: org.Name, Name: "package-repo", LowerName: "package-repo", IsPrivate: true}
	require.NoError(t, db.Insert(t.Context(), repo))
	require.NoError(t, db.Insert(t.Context(),
		&repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypeCode},
		&repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePackages},
	))
	pkg := &packages_model.Package{OwnerID: org.ID, Type: packages_model.TypeGeneric, Name: "package", LowerName: "package"}
	require.NoError(t, db.Insert(t.Context(), pkg))

	postLink := func(repoName string) int {
		ctx, resp := contexttest.MockContext(t, "privated_org/-/packages/settings/generic/package")
		contexttest.LoadUser(t, ctx, 5) // package writer of the org through team 17
		ctx.Package = &context.Package{
			Owner:      org,
			AccessMode: perm.AccessModeWrite,
			Descriptor: &packages_model.PackageDescriptor{Package: unittest.AssertExistsAndLoadBean(t, &packages_model.Package{ID: pkg.ID}), Owner: org},
		}
		packageSettingsPostActionLink(ctx, &forms.PackageSettingForm{Action: "link", RepoName: repoName})
		return resp.Code
	}
	linkedRepoID := func() int64 {
		return unittest.AssertExistsAndLoadBean(t, &packages_model.Package{ID: pkg.ID}).RepoID
	}

	assert.Equal(t, http.StatusBadRequest, postLink(repo.Name))
	assert.Zero(t, linkedRepoID())

	require.NoError(t, db.Insert(t.Context(), &organization.TeamRepo{OrgID: org.ID, TeamID: 17, RepoID: repo.ID}))
	assert.Equal(t, http.StatusOK, postLink(repo.Name))
	assert.Equal(t, repo.ID, linkedRepoID())

	// removing the link needs the same rights as the unlink API
	assert.Equal(t, http.StatusBadRequest, postLink(""))
	assert.Equal(t, repo.ID, linkedRepoID())
}
