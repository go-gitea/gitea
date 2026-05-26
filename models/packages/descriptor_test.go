// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages_test

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"strings"
	"testing"

	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageBatchLoaders(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	pkg, version := createPackageVersion(t, owner.ID, repo.ID, "batch-loader-package", "1.0.0", owner.ID)
	blobA := createPackageBlob(t, "batch-loader-a")
	blobB := createPackageBlob(t, "batch-loader-b")
	fileB := createPackageFile(t, version.ID, blobB.ID, "z-file.txt")
	fileA := createPackageFile(t, version.ID, blobA.ID, "a-file.txt")

	_, err := packages_model.InsertProperty(t.Context(), packages_model.PropertyTypePackage, pkg.ID, "package-key", "package-value")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeVersion, version.ID, "version-key", "version-value")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeFile, fileA.ID, "file-key", "file-value-a")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeFile, fileB.ID, "file-key", "file-value-b")
	require.NoError(t, err)

	packages, err := packages_model.GetPackagesByIDs(t.Context(), []int64{pkg.ID, pkg.ID})
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, pkg.Name, packages[pkg.ID].Name)

	files, err := packages_model.GetFilesByVersionIDs(t.Context(), []int64{version.ID})
	require.NoError(t, err)
	require.Len(t, files[version.ID], 2)
	assert.Equal(t, fileA.ID, files[version.ID][0].ID)
	assert.Equal(t, fileB.ID, files[version.ID][1].ID)

	blobs, err := packages_model.GetBlobsByIDs(t.Context(), []int64{blobA.ID, blobB.ID})
	require.NoError(t, err)
	assert.Equal(t, blobA.HashSHA256, blobs[blobA.ID].HashSHA256)
	assert.Equal(t, blobB.HashSHA256, blobs[blobB.ID].HashSHA256)

	packageProperties, err := packages_model.GetPropertiesByRefIDs(t.Context(), packages_model.PropertyTypePackage, []int64{pkg.ID})
	require.NoError(t, err)
	require.Len(t, packageProperties[pkg.ID], 1)
	assert.Equal(t, "package-value", packageProperties[pkg.ID][0].Value)

	fileProperties, err := packages_model.GetPropertiesByRefIDs(t.Context(), packages_model.PropertyTypeFile, []int64{fileA.ID, fileB.ID})
	require.NoError(t, err)
	require.Len(t, fileProperties[fileA.ID], 1)
	require.Len(t, fileProperties[fileB.ID], 1)
	assert.Equal(t, "file-value-a", fileProperties[fileA.ID][0].Value)
	assert.Equal(t, "file-value-b", fileProperties[fileB.ID][0].Value)
}

func TestGetPackageDescriptorsBatch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	creator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	pkgOne, versionOne := createPackageVersion(t, owner.ID, repo.ID, "descriptor-batch-one", "1.0.0", creator.ID)
	blobOne := createPackageBlob(t, "descriptor-batch-one")
	fileOne := createPackageFile(t, versionOne.ID, blobOne.ID, "artifact-one.txt")

	pkgTwo, versionTwo := createPackageVersion(t, owner.ID, 0, "descriptor-batch-two", "2.0.0", 0)
	blobTwo := createPackageBlob(t, "descriptor-batch-two")
	fileTwo := createPackageFile(t, versionTwo.ID, blobTwo.ID, "artifact-two.txt")

	_, err := packages_model.InsertProperty(t.Context(), packages_model.PropertyTypePackage, pkgOne.ID, "package-key", "package-one")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeVersion, versionOne.ID, "version-key", "version-one")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeFile, fileOne.ID, "file-key", "file-one")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypePackage, pkgTwo.ID, "package-key", "package-two")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeVersion, versionTwo.ID, "version-key", "version-two")
	require.NoError(t, err)
	_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeFile, fileTwo.ID, "file-key", "file-two")
	require.NoError(t, err)

	descriptors, err := packages_model.GetPackageDescriptors(t.Context(), []*packages_model.PackageVersion{versionOne, versionTwo})
	require.NoError(t, err)
	require.Len(t, descriptors, 2)

	assert.Equal(t, pkgOne.ID, descriptors[0].Package.ID)
	assert.Equal(t, owner.ID, descriptors[0].Owner.ID)
	assert.Equal(t, creator.ID, descriptors[0].Creator.ID)
	require.NotNil(t, descriptors[0].Repository)
	assert.Equal(t, repo.ID, descriptors[0].Repository.ID)
	assert.Equal(t, "package-one", descriptors[0].PackageProperties.GetByName("package-key"))
	assert.Equal(t, "version-one", descriptors[0].VersionProperties.GetByName("version-key"))
	require.Len(t, descriptors[0].Files, 1)
	assert.Equal(t, fileOne.ID, descriptors[0].Files[0].File.ID)
	assert.Equal(t, blobOne.ID, descriptors[0].Files[0].Blob.ID)
	assert.Equal(t, "file-one", descriptors[0].Files[0].Properties.GetByName("file-key"))

	assert.Equal(t, pkgTwo.ID, descriptors[1].Package.ID)
	assert.Equal(t, owner.ID, descriptors[1].Owner.ID)
	assert.Equal(t, user_model.GhostUserID, descriptors[1].Creator.ID)
	assert.Nil(t, descriptors[1].Repository)
	assert.Equal(t, "package-two", descriptors[1].PackageProperties.GetByName("package-key"))
	assert.Equal(t, "version-two", descriptors[1].VersionProperties.GetByName("version-key"))
	require.Len(t, descriptors[1].Files, 1)
	assert.Equal(t, fileTwo.ID, descriptors[1].Files[0].File.ID)
	assert.Equal(t, blobTwo.ID, descriptors[1].Files[0].Blob.ID)
	assert.Equal(t, "file-two", descriptors[1].Files[0].Properties.GetByName("file-key"))
}

func createPackageVersion(t *testing.T, ownerID, repoID int64, name, version string, creatorID int64) (*packages_model.Package, *packages_model.PackageVersion) {
	t.Helper()

	pkg, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   ownerID,
		RepoID:    repoID,
		Type:      packages_model.TypeGeneric,
		Name:      name,
		LowerName: strings.ToLower(name),
	})
	require.NoError(t, err)

	pv, err := packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
		PackageID:    pkg.ID,
		CreatorID:    creatorID,
		Version:      version,
		LowerVersion: strings.ToLower(version),
	})
	require.NoError(t, err)
	return pkg, pv
}

func createPackageBlob(t *testing.T, key string) *packages_model.PackageBlob {
	t.Helper()

	sha256Sum := sha256.Sum256([]byte(key))
	sha512Sum := sha512.Sum512([]byte(key))
	blob, _, err := packages_model.GetOrInsertBlob(t.Context(), &packages_model.PackageBlob{
		Size:       int64(len(key)),
		HashMD5:    hex.EncodeToString(sha256Sum[:16]),
		HashSHA1:   hex.EncodeToString(sha256Sum[:20]),
		HashSHA256: hex.EncodeToString(sha256Sum[:]),
		HashSHA512: hex.EncodeToString(sha512Sum[:]),
	})
	require.NoError(t, err)
	return blob
}

func createPackageFile(t *testing.T, versionID, blobID int64, name string) *packages_model.PackageFile {
	t.Helper()

	pf, err := packages_model.TryInsertFile(t.Context(), &packages_model.PackageFile{
		VersionID: versionID,
		BlobID:    blobID,
		Name:      name,
		LowerName: strings.ToLower(name),
	})
	require.NoError(t, err)
	return pf
}
