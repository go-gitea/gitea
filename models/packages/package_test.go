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

	for _, testCase := range []struct {
		version           packages_model.PackageVersion
		assertHasPackages assert.BoolAssertionFunc
	}{
		// A package with an internal package version gets automatically cleaned up and should return false
		{packages_model.PackageVersion{PackageID: p.ID, LowerVersion: "internal", IsInternal: true}, assert.False},
		// A package with a normal package version should return true
		{packages_model.PackageVersion{PackageID: p.ID, LowerVersion: "normal", IsInternal: false}, assert.True},
	} {
		pv, err := packages_model.GetOrInsertVersion(t.Context(), &testCase.version)
		assert.NotNil(t, pv)
		assert.NoError(t, err)

		has, err = packages_model.HasOwnerPackages(t.Context(), owner.ID)
		testCase.assertHasPackages(t, has)
		assert.NoError(t, err)
	}
}
