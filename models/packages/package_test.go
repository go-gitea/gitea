// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages_test

import (
	"testing"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestHasOwnerPackages(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	p, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		LowerName: "package",
	})
	assert.NotNil(t, p)
	assert.NoError(t, err)

	// A package without package versions gets automatically cleaned up and should return false
	has, err := packages_model.HasOwnerPackages(t.Context(), owner.ID)
	assert.False(t, has)
	assert.NoError(t, err)

	pv, err := packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
		PackageID:    p.ID,
		LowerVersion: "internal",
		IsInternal:   true,
	})
	assert.NotNil(t, pv)
	assert.NoError(t, err)

	// A package with an internal package version gets automatically cleaned up and should return false
	has, err = packages_model.HasOwnerPackages(t.Context(), owner.ID)
	assert.False(t, has)
	assert.NoError(t, err)

	pv, err = packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
		PackageID:    p.ID,
		LowerVersion: "normal",
		IsInternal:   false,
	})
	assert.NotNil(t, pv)
	assert.NoError(t, err)

	// A package with a normal package version should return true
	has, err = packages_model.HasOwnerPackages(t.Context(), owner.ID)
	assert.True(t, has)
	assert.NoError(t, err)

	// Team package checks
	hasTeam, err := packages_model.HasTeamPackages(t.Context(), owner.ID, 10)
	assert.False(t, hasTeam)
	assert.NoError(t, err)

	pTeam, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		TeamID:    10,
		LowerName: "team-pkg",
	})
	assert.NotNil(t, pTeam)
	assert.NoError(t, err)

	_, err = packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
		PackageID:    pTeam.ID,
		LowerVersion: "1.0.0",
		IsInternal:   false,
	})
	assert.NoError(t, err)

	hasTeam, err = packages_model.HasTeamPackages(t.Context(), owner.ID, 10)
	assert.True(t, hasTeam)
	assert.NoError(t, err)

	hasAny, err := packages_model.HasAnyPackages(t.Context(), owner.ID)
	assert.True(t, hasAny)
	assert.NoError(t, err)
}

func TestTeamPackages(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	teamID := int64(42)

	p1, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		TeamID:    0,
		Type:      packages_model.TypeGeneric,
		Name:      "common-pkg",
		LowerName: "common-pkg",
	})
	assert.NoError(t, err)
	assert.NotNil(t, p1)

	// Same name and type, but for a team: must succeed
	p2, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		TeamID:    teamID,
		Type:      packages_model.TypeGeneric,
		Name:      "common-pkg",
		LowerName: "common-pkg",
	})
	assert.NoError(t, err)
	assert.NotNil(t, p2)
	assert.NotEqual(t, p1.ID, p2.ID)

	// Duplicate in the same team: must fail
	_, err = packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		TeamID:    teamID,
		Type:      packages_model.TypeGeneric,
		Name:      "common-pkg",
		LowerName: "common-pkg",
	})
	assert.ErrorIs(t, err, packages_model.ErrDuplicatePackage)

	// Lookup by name
	foundOrg, err := packages_model.GetPackageByName(t.Context(), owner.ID, packages_model.TypeGeneric, "common-pkg")
	assert.NoError(t, err)
	assert.Equal(t, p1.ID, foundOrg.ID)

	foundTeam, err := packages_model.GetTeamPackageByName(t.Context(), owner.ID, teamID, packages_model.TypeGeneric, "common-pkg")
	assert.NoError(t, err)
	assert.Equal(t, p2.ID, foundTeam.ID)
}
