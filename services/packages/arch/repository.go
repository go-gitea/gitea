// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	packages_service "code.gitea.io/gitea/services/packages"
)

// Get data related to provided filename and distribution, for package files
// update download counter.
func GetPackageFile(ctx context.Context, distro, file string, ownderID int64) (io.ReadSeekCloser, error) {
	pf, err := getPackageFile(ctx, distro, file, ownderID)
	if err != nil {
		return nil, err
	}

	filestream, _, _, err := packages_service.GetPackageFileStream(ctx, pf)
	return filestream, err
}

// This function will search for package signature and if present, will load it
// from package file properties, and return its byte reader.
func GetPackageSignature(ctx context.Context, distro, file string, ownderID int64) (*bytes.Reader, error) {
	pf, err := getPackageFile(ctx, distro, strings.TrimSuffix(file, ".sig"), ownderID)
	if err != nil {
		return nil, err
	}

	proprs, err := packages_model.GetProperties(ctx, packages_model.PropertyTypeFile, pf.ID)
	if err != nil {
		return nil, err
	}

	for _, pp := range proprs {
		if pp.Name == arch_module.PropertySignature {
			b, err := base64.RawURLEncoding.DecodeString(pp.Value)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(b), nil
		}
	}

	return nil, errors.New("signature for requested package not found")
}

// Ejects parameters required to get package file property from file name.
func getPackageFile(ctx context.Context, distro, file string, ownderID int64) (*packages_model.PackageFile, error) {
	var (
		splt    = strings.Split(file, "-")
		pkgname = strings.Join(splt[0:len(splt)-3], "-")
		vername = splt[len(splt)-3] + "-" + splt[len(splt)-2]
	)

	version, err := packages_model.GetVersionByNameAndVersion(ctx, ownderID, packages_model.TypeArch, pkgname, vername)
	if err != nil {
		return nil, err
	}

	pkgfile, err := packages_model.GetFileForVersionByName(ctx, version.ID, file, distro)
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
func CreatePacmanDb(ctx context.Context, owner, arch, distro string, ownerID int64) (*bytes.Buffer, error) {
	pkgs, err := packages_model.GetPackagesByType(ctx, ownerID, packages_model.TypeArch)
	if err != nil {
		return nil, err
	}

	entries := make(map[string][]byte)

	for _, pkg := range pkgs {
		versions, err := packages_model.GetVersionsByPackageName(
			ctx, ownerID, packages_model.TypeArch, pkg.Name,
		)
		if err != nil {
			return nil, err
		}

		sort.Slice(versions, func(i, j int) bool {
			return versions[i].CreatedUnix > versions[j].CreatedUnix
		})

		for _, ver := range versions {
			file := fmt.Sprintf("%s-%s-%s.pkg.tar.zst", pkg.Name, ver.Version, arch)

			pf, err := packages_model.GetFileForVersionByName(ctx, ver.ID, file, distro)
			if err != nil {
				file = fmt.Sprintf("%s-%s-any.pkg.tar.zst", pkg.Name, ver.Version)
				pf, err = packages_model.GetFileForVersionByName(ctx, ver.ID, file, distro)
				if err != nil {
					continue
				}
			}

			pps, err := packages_model.GetPropertiesByName(
				ctx, packages_model.PropertyTypeFile, pf.ID, arch_module.PropertyDescription,
			)
			if err != nil {
				return nil, err
			}

			if len(pps) >= 1 {
				entries[pkg.Name+"-"+ver.Version+"/desc"] = []byte(pps[0].Value)
				break
			}
		}
	}

	return arch_module.CreatePacmanDb(entries)
}
