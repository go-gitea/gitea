// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"errors"
	"fmt"
	"io"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	alpine_module "code.gitea.io/gitea/modules/packages/alpine"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
)

// UploadAlpinePackage adds a Alpine Package to the registry. The first return value indictaes if the error is a user error.
func UploadAlpinePackage(ctx *context.Context, upload io.Reader, branch, repository string) (bool, *packages_model.PackageVersion, error) {
	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		return false, nil, err
	}
	defer buf.Close()

	pck, err := alpine_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) || err == io.EOF {
			return true, nil, err
		}

		return false, nil, err
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return false, nil, err
	}

	fileMetadataRaw, err := json.Marshal(pck.FileMetadata)
	if err != nil {
		return false, nil, err
	}

	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeAlpine,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:  ctx.Doer,
			Metadata: pck.VersionMetadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s-%s.apk", pck.Name, pck.Version),
				CompositeKey: fmt.Sprintf("%s|%s|%s", branch, repository, pck.FileMetadata.Architecture),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				alpine_module.PropertyBranch:       branch,
				alpine_module.PropertyRepository:   repository,
				alpine_module.PropertyArchitecture: pck.FileMetadata.Architecture,
				alpine_module.PropertyMetadata:     string(fileMetadataRaw),
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

	if err := BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, branch, repository, pck.FileMetadata.Architecture); err != nil {
		return false, nil, err
	}

	return false, pv, nil
}
