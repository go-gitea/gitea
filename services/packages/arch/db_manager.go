// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	pkg_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	pkg_service "code.gitea.io/gitea/services/packages"
)

// Parameters required to save new arch package.
type SaveFileParams struct {
	*org_model.Organization
	*user.User
	*arch.Metadata
	Data     []byte
	Filename string
	Distro   string
	IsLead   bool
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

	pv, _, err := pkg_service.CreatePackageOrAddFileToExisting(
		&pkg_service.PackageCreationInfo{
			PackageInfo: pkg_service.PackageInfo{
				Owner:       p.Organization.AsUser(),
				PackageType: pkg_model.TypeArch,
				Name:        p.Metadata.Name,
				Version:     p.Metadata.Version,
			},
			Creator:  p.User,
			Metadata: p.Metadata,
		},
		&pkg_service.PackageFileCreationInfo{
			PackageFileInfo: pkg_service.PackageFileInfo{
				Filename:     p.Filename,
				CompositeKey: p.Distro + "-" + p.Filename,
			},
			Creator:           p.User,
			Data:              buf,
			OverwriteExisting: true,
			IsLead:            p.IsLead,
		},
	)
	if err != nil {
		return 0, err
	}
	return pv.PackageID, nil
}

// Get data related to provided file name and distribution, and update download
// counter if actual package file is retrieved from database.
func LoadFile(ctx *context.Context, distro, file string) ([]byte, error) {
	db := db.GetEngine(ctx)

	pkgfile := &pkg_model.PackageFile{CompositeKey: distro + "-" + file}

	ok, err := db.Get(pkgfile)
	if err != nil || !ok {
		return nil, fmt.Errorf("%+v %t", err, ok)
	}

	blob, err := pkg_model.GetBlobByID(ctx, pkgfile.BlobID)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(file, ".pkg.tar.zst") {
		err = pkg_model.IncrementDownloadCounter(ctx, pkgfile.VersionID)
		if err != nil {
			return nil, err
		}
	}

	cs := packages.NewContentStore()

	obj, err := cs.Get(packages.BlobHash256Key(blob.HashSHA256))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(obj)
}

// Automatically connect repository to pushed package, if package with provided
// with provided name exists in namespace scope.
func RepositoryAutoconnect(ctx *context.Context, owner, repository string, pkgid int64) error {
	repo, err := repo.GetRepositoryByOwnerAndName(ctx, owner, repository)
	if err == nil {
		err = pkg_model.SetRepositoryLink(ctx, pkgid, repo.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// This function is collecting information about packages in some organization/
// user space, and created pacman database archive based on package metadata.
func CreatePacmanDb(ctx *context.Context, owner, architecture, distro string) ([]byte, error) {
	u, err := user.GetUserByName(ctx, owner)
	if err != nil {
		return nil, err
	}

	pkgs, err := pkg_model.GetPackagesByType(ctx, u.ID, pkg_model.TypeArch)
	if err != nil {
		return nil, err
	}

	var mds []*arch.Metadata

	for _, pkg := range pkgs {
		vers, err := pkg_model.GetVersionsByPackageName(ctx, u.ID, pkg_model.TypeArch, pkg.Name)
		if err != nil {
			return nil, err
		}
		for i := len(vers) - 1; i >= 0; i-- {
			var md arch.Metadata
			err = json.Unmarshal([]byte(vers[i].MetadataJSON), &md)
			if err != nil {
				return nil, err
			}
			if checkArchitecture(architecture, md.Arch) && md.Distribution == distro {
				mds = append(mds, &md)
				break
			}
		}
	}

	return arch.CreatePacmanDb(mds)
}

// Remove specific package version related to provided user or organization.
func RemovePackage(ctx *context.Context, u *user.User, name, version string) error {
	ver, err := pkg_model.GetVersionByNameAndVersion(ctx, u.ID, pkg_model.TypeArch, name, version)
	if err != nil {
		return err
	}

	return pkg_service.RemovePackageVersion(u, ver)
}

// Check wether package architecture is relevant for requestsed database.
func checkArchitecture(architecture string, list []string) bool {
	for _, v := range list {
		if v == "any" {
			return true
		}
		if v == architecture {
			return true
		}
	}
	return false
}
