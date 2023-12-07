// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	"code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPackage convert a packages.PackageDescriptor to api.Package
func ToPackage(ctx context.Context, pd *packages.PackageDescriptor, doer *user_model.User) (*api.Package, error) {
	var repo *api.Repository
	if pd.Repository != nil {
		permission, err := access_model.GetUserRepoPermission(ctx, pd.Repository, doer)
		if err != nil {
			return nil, err
		}

		if permission.HasAccess() {
			repo = ToRepo(ctx, pd.Repository, permission)
		}
	}

	return &api.Package{
		ID:         pd.Version.ID,
		Owner:      ToUser(ctx, pd.Owner, doer),
		Repository: repo,
		Creator:    ToUser(ctx, pd.Creator, doer),
		Type:       string(pd.Package.Type),
		Name:       pd.Package.Name,
		Version:    pd.Version.Version,
		CreatedAt:  pd.Version.CreatedUnix.AsTime(),
		HTMLURL:    pd.FullWebLink(),
	}, nil
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
