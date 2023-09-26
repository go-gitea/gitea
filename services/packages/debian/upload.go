// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"errors"
	"fmt"
	"io"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
)

// UploadDebianPackage adds a Debian Package to the registry. The first return value indictaes if the error is a user error.
func UploadDebianPackage(ctx *context.Context, upload io.Reader, distribution, component string) (bool, *packages_model.PackageVersion, error) {
	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		return false, nil, err
	}
	defer buf.Close()

	pck, err := debian_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			return true, nil, err
		}

		return false, nil, err
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return false, nil, err
	}

	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeDebian,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:  ctx.Doer,
			Metadata: pck.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s_%s_%s.deb", pck.Name, pck.Version, pck.Architecture),
				CompositeKey: fmt.Sprintf("%s|%s", distribution, component),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				debian_module.PropertyDistribution: distribution,
				debian_module.PropertyComponent:    component,
				debian_module.PropertyArchitecture: pck.Architecture,
				debian_module.PropertyControl:      pck.Control,
			},
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

	if err := BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, distribution, component, pck.Architecture); err != nil {
		return false, nil, err
	}

	return false, pv, nil
}
