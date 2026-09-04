// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	packages_module "gitea.dev/modules/packages"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// TestCreatePackageAndAddFileRestoresMissingBlobFile reproduces the state from
// https://github.com/go-gitea/gitea/issues/39215: a blob row exists in the
// database but its file in the content store is missing (e.g. after the file
// was lost on the storage). Publishing a package that references the same
// content must restore the missing blob file, otherwise every re-publish is a
// silent no-op and the package is permanently undownloadable.
func TestCreatePackageAndAddFileRestoresMissingBlobFile(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// A nupkg that only contains a zero-byte "_._" placeholder entry in
	// addition to the nuspec, mirroring the packages reported in the issue.
	// The same bytes are uploaded twice (as two different packages) so that
	// the second upload reuses the blob row from the first one.
	nupkg := test.WriteZipArchive(map[string]string{
		"package.nuspec":         "<package><metadata><id>nuget.repro</id><version>1.0.0</version></metadata></package>",
		"lib/netstandard2.0/_._": "",
	}).Bytes()
	nupkgSum := sha256.Sum256(nupkg)
	key := packages_module.BlobHash256Key(hex.EncodeToString(nupkgSum[:]))
	contentStore := packages_module.NewContentStore()

	// The initial upload writes the blob row and its file
	pf1, err := uploadPackage(t, user, "nuget.repro", "nuget.repro.1.0.0.nupkg", nupkg)
	require.NoError(t, err)
	assert.NoError(t, contentStore.Has(key))

	// Simulate the storage inconsistency: the blob row survives but its file
	// is missing (this is the state the issue reporter had, where deleting and
	// re-publishing the package never restored the file).
	require.NoError(t, contentStore.Delete(key))
	assert.Error(t, contentStore.Has(key))

	// Publishing a package with identical content must restore the blob file
	pf2, err := uploadPackage(t, user, "nuget.repro-copy", "nuget.repro-copy.1.0.0.nupkg", nupkg)
	require.NoError(t, err)

	// The blob file must be present and both packages must be downloadable
	assert.NoError(t, contentStore.Has(key))
	for _, pf := range []*packages_model.PackageFile{pf1, pf2} {
		s, _, _, err := OpenFileForDownload(t.Context(), pf, http.MethodGet)
		require.NoError(t, err)
		data, err := io.ReadAll(s)
		require.NoError(t, err)
		assert.NoError(t, s.Close())
		assert.Equal(t, nupkg, data)
	}
}

func uploadPackage(t *testing.T, user *user_model.User, name, filename string, data []byte) (*packages_model.PackageFile, error) {
	_, pf, err := CreatePackageAndAddFile(t.Context(),
		&PackageCreationInfo{
			PackageInfo: PackageInfo{
				Owner:       user,
				PackageType: packages_model.TypeNuGet,
				Name:        name,
				Version:     "1.0.0",
			},
			SemverCompatible: true,
			Creator:          user,
		},
		&PackageFileCreationInfo{
			PackageFileInfo: PackageFileInfo{Filename: filename},
			Creator:         user,
			Data:            mustHashedBuffer(t, data),
			IsLead:          true,
		})
	return pf, err
}

func mustHashedBuffer(t *testing.T, data []byte) *packages_module.HashedBuffer {
	buf, err := packages_module.CreateHashedBufferFromReader(bytes.NewReader(data))
	require.NoError(t, err)
	return buf
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
