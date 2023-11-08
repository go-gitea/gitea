// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"encoding/hex"
	"errors"
	"io"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	packages_service "code.gitea.io/gitea/services/packages"
)

// UploadArchPackage adds an Arch Package to the registry.
// The first return value indictaes if the error is a user error.
func UploadArchPackage(ctx *context.Context, upload io.Reader, filename, distro, sign string) (bool, *packages_model.PackageVersion, error) {
	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		return false, nil, err
	}
	defer buf.Close()

	md5, _, sha256, _ := buf.Sums()

	p, err := arch_module.ParsePackage(buf, md5, sha256, buf.Size())
	if err != nil {
		return false, nil, err
	}

	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		return false, nil, err
	}

	properties := map[string]string{
		arch_module.PropertyDescription: p.Desc(),
	}
	if sign != "" {
		_, err := hex.DecodeString(sign)
		if err != nil {
			return true, nil, errors.New("unable to decode package signature")
		}
		properties[arch_module.PropertySignature] = sign
	}

	ver, _, err := packages_service.CreatePackageOrAddFileToExisting(
		ctx, &packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeArch,
				Name:        p.Name,
				Version:     p.Version,
			},
			Creator:  ctx.Doer,
			Metadata: p.VersionMetadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     filename,
				CompositeKey: distro,
			},
			OverwriteExisting: true,
			IsLead:            true,
			Creator:           ctx.Doer,
			Data:              buf,
			Properties:        properties,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion, packages_model.ErrDuplicatePackageFile, packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			return true, nil, err
		default:
			return false, nil, err
		}
	}

	return false, ver, nil
}
