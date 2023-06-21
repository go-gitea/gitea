// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"errors"
	"fmt"
	"strings"

	org "code.gitea.io/gitea/models/organization"
	pkg "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/timeutil"
)

// This function will create new package in database, if it does not exist it
// will get existing and return it back to user.
func CreateGetPackage(ctx *context.Context, o *org.Organization, name string) (*pkg.Package, error) {
	pack, err := pkg.TryInsertPackage(ctx, &pkg.Package{
		OwnerID:   o.ID,
		Type:      pkg.TypeArch,
		Name:      name,
		LowerName: strings.ToLower(name),
	})
	if errors.Is(err, pkg.ErrDuplicatePackage) {
		pack, err = pkg.GetPackageByName(ctx, o.ID, pkg.TypeArch, name)
		if err != nil {
			return nil, fmt.Errorf("unable to get package %s in organization %s", name, o.Name)
		}
	}
	if err != nil {
		return nil, err
	}
	return pack, nil
}

// This function will create new version for package, or find and return existing.
func CreateGetPackageVersion(ctx *context.Context, md *arch.Metadata, p *pkg.Package, u *user.User) (*pkg.PackageVersion, error) {
	rawjsonmetadata, err := json.Marshal(&md)
	if err != nil {
		return nil, err
	}

	ver, err := pkg.GetOrInsertVersion(ctx, &pkg.PackageVersion{
		PackageID:    p.ID,
		CreatorID:    u.ID,
		Version:      md.Version,
		LowerVersion: strings.ToLower(md.Version),
		CreatedUnix:  timeutil.TimeStampNow(),
		MetadataJSON: string(rawjsonmetadata),
	})
	if err != nil {
		if errors.Is(err, pkg.ErrDuplicatePackageVersion) {
			return ver, nil
		}
		return nil, err
	}
	return ver, nil
}

// Automatically connect repository to pushed package, if package with provided
// with provided name exists in namespace scope.
func RepositoryAutoconnect(ctx *context.Context, owner, repository string, p *pkg.Package) error {
	repo, err := repo.GetRepositoryByOwnerAndName(ctx, owner, repository)
	if err == nil {
		err = pkg.SetRepositoryLink(ctx, p.ID, repo.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
