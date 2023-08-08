// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	pkg_model "code.gitea.io/gitea/models/packages"
	repository "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/storage"
)

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

type DbParams struct {
	Owner        string
	Architecture string
	Distribution string
}

// Finds all arch packages in user/organization scope, each package version
// starting from latest in descending order is checked to be compatible with
// requested combination of architecture and distribution. When/If the first
// compatible version is found, related desc file will be loaded from object
// storage and added to database archive.
func CreatePacmanDb(ctx *context.Context, p *DbParams) ([]byte, error) {
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
			p, err := pkg_model.GetPropertieWithUniqueName(ctx, fmt.Sprintf(
				"%s-%s-%s-%s.pkg.tar.zst.desc",
				p.Distribution, pkg.Name, version.Version, p.Architecture,
			))
			if err != nil {
				return nil, err
			}

			entries[pkg.Name+"-"+version.Version+"/desc"] = []byte(p.Value)
			break
		}
	}

	return arch.CreatePacmanDb(entries)
}
