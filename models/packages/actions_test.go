// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages_test

import (
	"testing"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/require"
)

func TestActionsPackageCreation(t *testing.T) {
	for _, packageType := range packages_model.TypeList {
		t.Run(string(packageType), func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			ctx := reqctx.NewRequestContextForTest(t)
			packages_model.SetActionsPackageAccess(ctx, 2, 1, perm.AccessModeWrite)
			pkg, err := packages_model.TryInsertPackage(ctx, &packages_model.Package{OwnerID: 2, Type: packageType, Name: "actions-package", LowerName: "actions-package"})
			require.NoError(t, err)
			require.EqualValues(t, 1, pkg.RepoID)
			version, err := packages_model.GetOrInsertVersion(ctx, &packages_model.PackageVersion{PackageID: pkg.ID, Version: "1.0.0", LowerVersion: "1.0.0"})
			require.NoError(t, err)
			_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, version.ID, "tag", "latest")
			require.NoError(t, err)
			packages_model.SetActionsPackageAccess(ctx, 2, 2, perm.AccessModeWrite)
			_, err = packages_model.TryInsertPackage(ctx, &packages_model.Package{OwnerID: 2, Type: packageType, Name: pkg.Name, LowerName: pkg.LowerName})
			require.ErrorIs(t, err, util.ErrPermissionDenied)
			_, err = packages_model.GetOrInsertVersion(ctx, &packages_model.PackageVersion{PackageID: pkg.ID, Version: "2.0.0", LowerVersion: "2.0.0"})
			require.ErrorIs(t, err, util.ErrPermissionDenied)
			require.ErrorIs(t, packages_model.UpdateVersion(ctx, version), util.ErrPermissionDenied)
			require.ErrorIs(t, packages_model.DeleteVersionByID(ctx, version.ID), util.ErrPermissionDenied)
			require.ErrorIs(t, packages_model.DeletePackageByID(ctx, pkg.ID), util.ErrPermissionDenied)
			_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, version.ID, "tag", "other")
			require.ErrorIs(t, err, util.ErrPermissionDenied)
			packages_model.SetActionsPackageAccess(ctx, 2, 1, perm.AccessModeRead)
			require.ErrorIs(t, packages_model.DeleteVersionByID(ctx, version.ID), util.ErrPermissionDenied)
			packages_model.SetActionsPackageAccess(ctx, 3, 1, perm.AccessModeWrite)
			require.ErrorIs(t, packages_model.DeleteVersionByID(ctx, version.ID), util.ErrPermissionDenied)
			packages_model.SetActionsPackageAccess(ctx, 2, 1, perm.AccessModeWrite)
			require.NoError(t, packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeVersion, version.ID))
			require.NoError(t, packages_model.DeleteVersionByID(ctx, version.ID))
			require.NoError(t, packages_model.DeletePackageByID(ctx, pkg.ID))
		})
	}
}

func TestActionsPackageRepositoryLink(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pkg, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{OwnerID: 2, Type: packages_model.TypeGeneric, Name: "unlinked", LowerName: "unlinked"})
	require.NoError(t, err)
	require.Zero(t, pkg.RepoID)
	ctx := reqctx.NewRequestContextForTest(t)
	packages_model.SetActionsPackageAccess(ctx, 2, 1, perm.AccessModeWrite)
	pkg, err = packages_model.TryInsertPackage(ctx, pkg)
	require.ErrorIs(t, err, packages_model.ErrDuplicatePackage)
	require.EqualValues(t, 1, pkg.RepoID)
	require.ErrorIs(t, packages_model.SetRepositoryLink(ctx, pkg.ID, 2), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.UnlinkRepository(ctx, pkg.ID), util.ErrPermissionDenied)
	require.NoError(t, packages_model.SetRepositoryLink(ctx, pkg.ID, 1))
	packages_model.SetActionsPackageAccess(ctx, 2, 2, perm.AccessModeWrite)
	require.ErrorIs(t, packages_model.SetRepositoryLink(ctx, pkg.ID, 2), util.ErrPermissionDenied)
	stored, err := packages_model.GetPackageByID(t.Context(), pkg.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.RepoID)
}

func TestActionsPackageFilesAndProperties(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := reqctx.NewRequestContextForTest(t)
	packages_model.SetActionsPackageAccess(ctx, 2, 1, perm.AccessModeWrite)
	pkg, err := packages_model.TryInsertPackage(ctx, &packages_model.Package{OwnerID: 2, Type: packages_model.TypeGeneric, Name: "files", LowerName: "files"})
	require.NoError(t, err)
	version, err := packages_model.GetOrInsertVersion(ctx, &packages_model.PackageVersion{PackageID: pkg.ID, Version: "1", LowerVersion: "1"})
	require.NoError(t, err)
	file, err := packages_model.TryInsertFile(ctx, &packages_model.PackageFile{VersionID: version.ID, Name: "test.txt", LowerName: "test.txt"})
	require.NoError(t, err)
	property, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeFile, file.ID, "key", "value")
	require.NoError(t, err)
	packages_model.SetActionsPackageAccess(ctx, 2, 2, perm.AccessModeWrite)
	require.ErrorIs(t, packages_model.UpdateFile(ctx, file, []string{"is_lead"}), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.DeleteFileByID(ctx, file.ID), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.UpdateProperty(ctx, property), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.DeletePropertyByID(ctx, property.ID), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.DeletePropertiesByName(ctx, property.RefType, property.RefID, property.Name), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.InsertOrUpdateProperty(ctx, property.RefType, property.RefID, property.Name, "changed"), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.DeleteFilesByPackageID(ctx, pkg.ID), util.ErrPermissionDenied)
	require.ErrorIs(t, packages_model.DeleteVersionsByPackageID(ctx, pkg.ID), util.ErrPermissionDenied)
	packages_model.SetActionsPackageAccess(ctx, 2, 1, perm.AccessModeWrite)
	require.NoError(t, packages_model.DeletePropertyByID(ctx, property.ID))
	require.NoError(t, packages_model.DeleteFileByID(ctx, file.ID))
}
