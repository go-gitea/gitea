// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"testing"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	packages_module "code.gitea.io/gitea/modules/packages"
	packages_service "code.gitea.io/gitea/services/packages"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemovePackage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// 1. Setup: Create two packages with properties at all levels
	createPackage := func(name string) (*packages_model.Package, *packages_model.PackageVersion, *packages_model.PackageFile) {
		data, _ := packages_module.CreateHashedBufferFromReader(bytes.NewReader([]byte{1}))
		pv, pf, err := packages_service.CreatePackageOrAddFileToExisting(t.Context(), &packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       user,
				PackageType: packages_model.TypeGeneric,
				Name:        name,
				Version:     "1.0.0",
			},
			Creator:           user,
			PackageProperties: map[string]string{"pkg_prop": "val"},
			VersionProperties: map[string]string{"ver_prop": "val"},
		}, &packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{Filename: "file.bin"},
			Creator:         user,
			Data:            data,
			Properties:      map[string]string{"file_prop": "val"},
		})
		require.NoError(t, err)

		p, err := packages_model.GetPackageByID(t.Context(), pv.PackageID)
		require.NoError(t, err)

		return p, pv, pf
	}

	p1, pv1, pf1 := createPackage("package-1")
	p2, pv2, pf2 := createPackage("package-2")

	// Verify properties exist before deletion
	checkProps := func(p *packages_model.Package, pv *packages_model.PackageVersion, pf *packages_model.PackageFile, shouldExist bool) {
		pps, err := packages_model.GetProperties(t.Context(), packages_model.PropertyTypePackage, p.ID)
		require.NoError(t, err)
		if shouldExist {
			assert.NotEmpty(t, pps)
		} else {
			assert.Empty(t, pps)
		}

		pps, err = packages_model.GetProperties(t.Context(), packages_model.PropertyTypeVersion, pv.ID)
		require.NoError(t, err)
		if shouldExist {
			assert.NotEmpty(t, pps)
		} else {
			assert.Empty(t, pps)
		}

		pps, err = packages_model.GetProperties(t.Context(), packages_model.PropertyTypeFile, pf.ID)
		require.NoError(t, err)
		if shouldExist {
			assert.NotEmpty(t, pps)
		} else {
			assert.Empty(t, pps)
		}
	}

	checkProps(p1, pv1, pf1, true)
	checkProps(p2, pv2, pf2, true)

	// 2. Act: Remove package 1
	err := packages_service.RemovePackage(t.Context(), user, p1)
	assert.NoError(t, err)

	// 3. Assert: Package 1 is gone, Package 2 is untouched

	// Check P1
	_, err = packages_model.GetPackageByID(t.Context(), p1.ID)
	assert.ErrorIs(t, err, packages_model.ErrPackageNotExist)

	_, err = packages_model.GetVersionByID(t.Context(), pv1.ID)
	assert.ErrorIs(t, err, packages_model.ErrPackageNotExist)

	_, err = packages_model.GetFileForVersionByID(t.Context(), pv1.ID, pf1.ID)
	assert.ErrorIs(t, err, packages_model.ErrPackageFileNotExist)

	checkProps(p1, pv1, pf1, false)

	// Check P2
	p2After, err := packages_model.GetPackageByID(t.Context(), p2.ID)
	assert.NoError(t, err)
	assert.NotNil(t, p2After)

	pv2After, err := packages_model.GetVersionByID(t.Context(), pv2.ID)
	assert.NoError(t, err)
	assert.NotNil(t, pv2After)

	pf2After, err := packages_model.GetFileForVersionByID(t.Context(), pv2.ID, pf2.ID)
	assert.NoError(t, err)
	assert.NotNil(t, pf2After)

	checkProps(p2, pv2, pf2, true)
}
