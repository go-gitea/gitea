// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPackage convert a packages.PackageDescriptor to api.Package
func ToPackage(pd *packages.PackageDescriptor) *api.Package {
	var repo *api.Repository
	if pd.Repository != nil {
		repo = ToRepo(pd.Repository, perm.AccessModeNone)
	}

	return &api.Package{
		ID:         pd.Version.ID,
		Owner:      ToUser(pd.Owner, nil),
		Repository: repo,
		Creator:    ToUser(pd.Creator, nil),
		Type:       string(pd.Package.Type),
		Name:       pd.Package.Name,
		Version:    pd.Version.Version,
		CreatedAt:  pd.Version.CreatedUnix.AsTime(),
	}
}

// ToPackageFile converts packages.PackageFileDescriptor to api.PackageFile
func ToPackageFile(pfd *packages.PackageFileDescriptor) *api.PackageFile {
	return &api.PackageFile{
		ID:         pfd.File.ID,
		Size:       pfd.Blob.Size,
		Name:       pfd.File.Name,
		HashMD5:    pfd.Blob.HashMD5,
		HashSHA1:   pfd.Blob.HashSHA1,
		HashSHA256: pfd.Blob.HashSHA256,
		HashSHA512: pfd.Blob.HashSHA512,
	}
}
