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
	repository "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/storage"
	pkg_service "code.gitea.io/gitea/services/packages"
)

type UpdateMetadataParameters struct {
	User     *user.User
	Metadata *arch.Metadata
	DbDesc   *arch.DbDesc
}

// Update package metadata stored in SQL database with new combination of
// distribution and architecture.
func UpdateMetadata(ctx *context.Context, p *UpdateMetadataParameters) error {
	ver, err := pkg_model.GetVersionByNameAndVersion(
		ctx,
		p.User.ID,
		pkg_model.TypeArch,
		p.DbDesc.Name,
		p.DbDesc.Version,
	)
	if err != nil {
		return err
	}

	var currmd arch.Metadata
	err = json.Unmarshal([]byte(ver.MetadataJSON), &currmd)
	if err != nil {
		return err
	}

	currmd.DistroArch = uniqueSlice(currmd.DistroArch, p.Metadata.DistroArch)

	b, err := json.Marshal(&currmd)
	if err != nil {
		return err
	}

	ver.MetadataJSON = string(b)

	return pkg_model.UpdateVersion(ctx, ver)
}

// Creates a list containing unique values formed of 2 passed slices.
func uniqueSlice(first, second []string) []string {
	set := make(container.Set[string], len(first)+len(second))
	set.AddMultiple(first...)
	set.AddMultiple(second...)
	return set.Values()
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
	PkgName  string
	PkgVer   string
}

// Creates new package, version and package_file properties in database,
// and writes blob to file storage. If package/version/blob exists it
// will overwrite existing data. Package id and error will be returned.
func SaveFile(ctx *context.Context, p *SaveFileParams) (int64, error) {
	ver, _, err := pkg_service.CreatePackageOrAddFileToExisting(
		&pkg_service.PackageCreationInfo{
			PackageInfo: pkg_service.PackageInfo{
				Owner:       p.Owner,
				PackageType: pkg_model.TypeArch,
				Name:        p.PkgName,
				Version:     p.PkgVer,
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

// Get data related to provided filename and distribution, for package files
// update download counter.
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

// Automatically connect repository with source code to published package, if
// repository with the same name exists in user/organization scope.
func RepoConnect(ctx *context.Context, owner, repo string, pkgid int64) error {
	r, err := repository.GetRepositoryByOwnerAndName(ctx, owner, repo)
	if err == nil {
		err = pkg_model.SetRepositoryLink(ctx, pkgid, r.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

type PacmanDbParams struct {
	Owner        string
	Architecture string
	Distribution string
}

// Finds all arch packages in user/organization scope, each package version
// starting from latest in descending order is checked to be compatible with
// requested combination of architecture and distribution. When/If the first
// compatible version is found, related desc file will be loaded from object
// storage and added to database archive.
func CreatePacmanDb(ctx *context.Context, p *PacmanDbParams) ([]byte, error) {
	u, err := user.GetUserByName(ctx, p.Owner)
	if err != nil {
		return nil, err
	}

	pkgs, err := pkg_model.GetPackagesByType(ctx, u.ID, pkg_model.TypeArch)
	if err != nil {
		return nil, err
	}

	entries := make(map[string][]byte)

	for _, pkg := range pkgs {
		versions, err := pkg_model.GetVersionsByPackageName(
			ctx, u.ID, pkg_model.TypeArch, pkg.Name,
		)
		if err != nil {
			return nil, err
		}

		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreatedUnix > versions[j].CreatedUnix
		})

		for _, version := range versions {
			desc, err := LoadDbDescFile(ctx, &DescParams{
				Version: version,
				Arch:    p.Architecture,
				Distro:  p.Distribution,
				PkgName: pkg.Name,
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
	PkgName string
}

// Get pacman desc file from object storage if combination of distribution and
// architecture is supported (checked in metadata).
func LoadDbDescFile(ctx *context.Context, p *DescParams) ([]byte, error) {
	var md arch.Metadata
	err := json.Unmarshal([]byte(p.Version.MetadataJSON), &md)
	if err != nil {
		return nil, err
	}

	for _, distroarch := range md.DistroArch {
		var file string

		if distroarch == p.Distro+"-"+p.Arch {
			file = p.PkgName + "-" + p.Version.Version + "-" + p.Arch + ".desc"
		}
		if distroarch == p.Distro+"-any" {
			file = p.PkgName + "-" + p.Version.Version + "-any.desc"
		}

		if file == "" {
			continue
		}

		descfile, err := GetFileObject(ctx, p.Distro, file)
		if err != nil {
			return nil, err
		}

		return io.ReadAll(descfile)
	}
	return nil, nil
}
