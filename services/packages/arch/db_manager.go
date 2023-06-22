// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"

	org "code.gitea.io/gitea/models/organization"
	pkg "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	svc "code.gitea.io/gitea/services/packages"
)

// Parameters required to save new arch package.
type SaveFileParams struct {
	*org.Organization
	*user.User
	*arch.Metadata
	Data     []byte
	Filename string
	Distro   string
}

// This function create new package, version and package file properties in
// database, and write blob to file storage. If package/version/blob exists it
// will overwrite existing data. Package id and error will be returned.
func SaveFile(ctx *context.Context, p *SaveFileParams) (int64, error) {
	buf, err := packages.CreateHashedBufferFromReader(bytes.NewReader(p.Data))
	if err != nil {
		return 0, err
	}
	defer buf.Close()

	pv, _, err := svc.CreatePackageOrAddFileToExisting(
		&svc.PackageCreationInfo{
			PackageInfo: svc.PackageInfo{
				Owner:       p.Organization.AsUser(),
				PackageType: pkg.TypeArch,
				Name:        p.Metadata.Name,
				Version:     p.Metadata.Version,
			},
			Creator:  p.User,
			Metadata: p.Metadata,
		},
		&svc.PackageFileCreationInfo{
			PackageFileInfo: svc.PackageFileInfo{
				Filename:     p.Filename,
				CompositeKey: p.Distro + "-" + p.Filename,
			},
			Creator:           p.User,
			Data:              buf,
			OverwriteExisting: true,
		},
	)
	if err != nil {
		return 0, err
	}
	return pv.PackageID, nil
}

// Automatically connect repository to pushed package, if package with provided
// with provided name exists in namespace scope.
func RepositoryAutoconnect(ctx *context.Context, owner, repository string, pkgid int64) error {
	repo, err := repo.GetRepositoryByOwnerAndName(ctx, owner, repository)
	if err == nil {
		err = pkg.SetRepositoryLink(ctx, pkgid, repo.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
