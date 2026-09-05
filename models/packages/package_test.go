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
}

func TestCountOwnerPackages(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	count, err := packages_model.CountOwnerPackages(t.Context(), owner.ID)
	assert.EqualValues(t, 0, count)
	assert.NoError(t, err)

	p1, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		LowerName: "package-1",
	})
	assert.NotNil(t, p1)
	assert.NoError(t, err)

	count, err = packages_model.CountOwnerPackages(t.Context(), owner.ID)
	assert.EqualValues(t, 0, count)
	assert.NoError(t, err)

	pv, err := packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
		PackageID:    p1.ID,
		LowerVersion: "internal",
		IsInternal:   true,
	})
	assert.NotNil(t, pv)
	assert.NoError(t, err)

	count, err = packages_model.CountOwnerPackages(t.Context(), owner.ID)
	assert.EqualValues(t, 0, count)
	assert.NoError(t, err)

	for _, version := range []string{"1.0.0", "1.0.1"} {
		pv, err = packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
			PackageID:    p1.ID,
			LowerVersion: version,
		})
		assert.NotNil(t, pv)
		assert.NoError(t, err)
	}

	count, err = packages_model.CountOwnerPackages(t.Context(), owner.ID)
	assert.EqualValues(t, 1, count)
	assert.NoError(t, err)

	p2, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   owner.ID,
		LowerName: "package-2",
	})
	assert.NotNil(t, p2)
	assert.NoError(t, err)

	pv, err = packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
		PackageID:    p2.ID,
		LowerVersion: "1.0.0",
	})
	assert.NotNil(t, pv)
	assert.NoError(t, err)

	count, err = packages_model.CountOwnerPackages(t.Context(), owner.ID)
	assert.EqualValues(t, 2, count)
	assert.NoError(t, err)
}
