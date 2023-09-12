// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package generic

import (
	"io"
	"net/http"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	packages_service "code.gitea.io/gitea/services/packages"
)

// UploadGenericPackage adds a Generic Package to the registry
func UploadGenericPackage(ctx *context.Context, upload io.Reader, name, version, filename string) (int, *packages_model.PackageVersion, error) {
	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		ctx.ServerError("CreateHashedBufferFromReader", err)
		return http.StatusInternalServerError, nil, err
	}
	defer buf.Close()

	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeGeneric,
				Name:        name,
				Version:     version,
			},
			Creator: ctx.Doer,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			return http.StatusConflict, nil, err
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			return http.StatusForbidden, nil, err
		default:
			return http.StatusInternalServerError, nil, err
		}
	}

	return http.StatusCreated, pv, nil
}
