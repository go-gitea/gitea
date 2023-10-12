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

	desc, err := arch_module.ParseMetadata(filename, distro, buf)
	if err != nil {
		return false, nil, err
	}

	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		return false, nil, err
	}

	properties := map[string]string{
		"desc": desc.String(),
	}
	if sign != "" {
		_, err := hex.DecodeString(sign)
		if err != nil {
			return true, nil, errors.New("unable to decode package signature")
		}
		properties["sign"] = sign
	}

	ver, _, err := packages_service.CreatePackageOrAddFileToExisting(
		ctx, &packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeArch,
				Name:        desc.Name,
				Version:     desc.Version,
			},
			Creator: ctx.Doer,
			Metadata: &arch_module.Metadata{
				URL:          desc.ProjectURL,
				Description:  desc.Description,
				Provides:     desc.Provides,
				License:      desc.License,
				Depends:      desc.Depends,
				OptDepends:   desc.OptDepends,
				MakeDepends:  desc.MakeDepends,
				CheckDepends: desc.CheckDepends,
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     filename,
				CompositeKey: distro,
			},
			OverwriteExisting: true,
			IsLead:            true,
			Creator:           ctx.ContextUser,
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
