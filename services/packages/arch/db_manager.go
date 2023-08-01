// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	pkg_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/storage"
	pkg_service "code.gitea.io/gitea/services/packages"
)

type UpdateMetadataParameters struct {
	User *user.User
	Md   *arch.Metadata
}

// This function gets existing package metadata for provided version present,
// combines architecture and distribution info and creates new metadata with
// combined set of parameters.
func UpdateMetadata(ctx *context.Context, p *UpdateMetadataParameters) error {
	ver, err := pkg_model.GetVersionByNameAndVersion(ctx, p.User.ID, pkg_model.TypeArch, p.Md.Name, p.Md.Version)
	if err != nil {
		return err
	}

	var currmd arch.Metadata
	err = json.Unmarshal([]byte(ver.MetadataJSON), &currmd)
	if err != nil {
		return err
	}

	currmd.DistroArch = arch.UnifiedList(currmd.DistroArch, p.Md.DistroArch)

	b, err := json.Marshal(&currmd)
	if err != nil {
		return err
	}

	ver.MetadataJSON = string(b)

	return pkg_model.UpdateVersion(ctx, ver)
}

// Parameters required to save new arch package.
type SaveFileParams struct {
	Creator  *user.User
	Owner    *user.User
	Metadata *arch.Metadata
	Buf      packages.HashedSizeReader
	Filename string
	Distro   string
	IsLead   bool
}

// This function creates new package, version and package_file properties in
// database, and writes blob to file storage. If package/version/blob exists it
// will overwrite existing data. Package id and error will be returned.
func SaveFile(ctx *context.Context, p *SaveFileParams) (int64, error) {
	ver, _, err := pkg_service.CreatePackageOrAddFileToExisting(
		&pkg_service.PackageCreationInfo{
			PackageInfo: pkg_service.PackageInfo{
				Owner:       p.Owner,
				PackageType: pkg_model.TypeArch,
				Name:        p.Metadata.Name,
				Version:     p.Metadata.Version,
			},
			Creator:  p.Creator,
			Metadata: p.Metadata,
		},
		&pkg_service.PackageFileCreationInfo{
			PackageFileInfo: pkg_service.PackageFileInfo{
				Filename:     p.Filename,
				CompositeKey: p.Distro + "-" + p.Filename,
			},
			Creator:           p.Creator,
			Data:              p.Buf,
			OverwriteExisting: true,
			IsLead:            p.IsLead,
		},
	)
	if err != nil {
		return 0, err
	}
	return ver.ID, nil
}

// Get data related to provided file name and distribution, and update download
// counter if actual package file is retrieved from database.
func GetFileObject(ctx *context.Context, distro, file string) (storage.Object, error) {
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

	return cs.Get(packages.BlobHash256Key(blob.HashSHA256))
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

	var entries = make(map[string][]byte)

	for _, pkg := range pkgs {
		versions, err := pkg_model.GetVersionsByPackageName(ctx, u.ID, pkg_model.TypeArch, pkg.Name)
		if err != nil {
			return nil, err
		}

		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreatedUnix > versions[j].CreatedUnix
		})

		for _, version := range versions {
			desc, err := GetPacmanDbDesc(ctx, &DescParams{
				Version: version,
				Arch:    architecture,
				Distro:  distro,
			})
			if err != nil {
				return nil, err
			}
			if desc == nil {
				continue
			}
			entries[pkg.Name+"-"+version.Version+"/desc"] = desc
			break
		}
	}

	return arch.CreatePacmanDb(entries)
}

type DescParams struct {
	Version *pkg_model.PackageVersion
	Arch    string
	Distro  string
}

// Checks if desc file exists for required architecture or any and returns it
// in form of byte slice.
func GetPacmanDbDesc(ctx *context.Context, p *DescParams) ([]byte, error) {
	var md arch.Metadata
	err := json.Unmarshal([]byte(p.Version.MetadataJSON), &md)
	if err != nil {
		return nil, err
	}

	for _, distroarch := range md.DistroArch {
		var storagekey string

		if distroarch == p.Distro+"-"+p.Arch {
			storagekey = md.Name + "-" + md.Version + "-" + p.Arch + ".desc"
		}
		if distroarch == p.Distro+"-any" {
			storagekey = md.Name + "-" + md.Version + "-any.desc"
		}

		if storagekey == "" {
			continue
		}

		descfile, err := GetFileObject(ctx, p.Distro, storagekey)
		if err != nil {
			return nil, err
		}

		return io.ReadAll(descfile)
	}
	return nil, nil
}

// Remove specific package version related to provided user or organization.
func RemovePackage(ctx *context.Context, u *user.User, name, version string) error {
	ver, err := pkg_model.GetVersionByNameAndVersion(ctx, u.ID, pkg_model.TypeArch, name, version)
	if err != nil {
		return err
	}

	return pkg_service.RemovePackageVersion(u, ver)
}
