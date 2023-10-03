// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	pkg_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	pkg_service "code.gitea.io/gitea/services/packages"
)

// Get data related to provided filename and distribution, for package files
// update download counter.
func GetPackageFile(ctx *context.Context, distro, file string) (io.ReadSeekCloser, error) {
	pf, err := getPackageFile(ctx, distro, file)
	if err != nil {
		return nil, err
	}

	filestream, _, _, err := pkg_service.GetPackageFileStream(ctx, pf)
	return filestream, err
}

// This function will search for package signature and if present, will load it
// from package file properties, and return its byte reader.
func GetPackageSignature(ctx *context.Context, distro, file string) (*bytes.Reader, error) {
	pf, err := getPackageFile(ctx, distro, strings.TrimSuffix(file, ".sig"))
	if err != nil {
		return nil, err
	}

	proprs, err := pkg_model.GetProperties(ctx, pkg_model.PropertyTypeFile, pf.ID)
	if err != nil {
		return nil, err
	}

	for _, pp := range proprs {
		if pp.Name == "sign" {
			b, err := hex.DecodeString(pp.Value)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(b), nil
		}
	}

	return nil, errors.New("signature for requested package not found")
}

// Ejects parameters required to get package file property from file name.
func getPackageFile(ctx *context.Context, distro, file string) (*pkg_model.PackageFile, error) {
	var (
		splt    = strings.Split(file, "-")
		pkgname = strings.Join(splt[0:len(splt)-3], "-")
		vername = splt[len(splt)-3] + "-" + splt[len(splt)-2]
	)

	version, err := pkg_model.GetVersionByNameAndVersion(
		ctx, ctx.Package.Owner.ID, pkg_model.TypeArch, pkgname, vername,
	)
	if err != nil {
		return nil, err
	}

	pkgfile, err := pkg_model.GetFileForVersionByName(ctx, version.ID, file, distro)
	if err != nil {
		return nil, err
	}
	return pkgfile, nil
}

// Finds all arch packages in user/organization scope, each package version
// starting from latest in descending order is checked to be compatible with
// requested combination of architecture and distribution. When/If the first
// compatible version is found, related desc file will be loaded from package
// properties and added to resulting .db.tar.gz archive.
func CreatePacmanDb(ctx *context.Context, owner, arch, distro string) (io.ReadSeeker, error) {
	pkgs, err := pkg_model.GetPackagesByType(ctx, ctx.Package.Owner.ID, pkg_model.TypeArch)
	if err != nil {
		return nil, err
	}

	entries := make(map[string][]byte)

	for _, pkg := range pkgs {
		versions, err := pkg_model.GetVersionsByPackageName(
			ctx, ctx.Package.Owner.ID, pkg_model.TypeArch, pkg.Name,
		)
		if err != nil {
			return nil, err
		}

		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreatedUnix > versions[j].CreatedUnix
		})

		for _, ver := range versions {
			file := fmt.Sprintf("%s-%s-%s.pkg.tar.zst", pkg.Name, ver.Version, arch)

			pf, err := pkg_model.GetFileForVersionByName(ctx, ver.ID, file, distro)
			if err != nil {
				file = fmt.Sprintf("%s-%s-any.pkg.tar.zst", pkg.Name, ver.Version)
				pf, err = pkg_model.GetFileForVersionByName(ctx, ver.ID, file, distro)
				if err != nil {
					continue
				}
			}

			pps, err := pkg_model.GetProperties(ctx, pkg_model.PropertyTypeFile, pf.ID)
			if err != nil {
				return nil, err
			}

			for _, pp := range pps {
				if pp.Name == "desc" {
					entries[pkg.Name+"-"+ver.Version+"/desc"] = []byte(pp.Value)
					break
				}
			}
		}
	}

	return arch_module.CreatePacmanDb(entries)
}
