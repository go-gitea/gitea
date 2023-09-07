// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
	rpm_service "code.gitea.io/gitea/services/packages/rpm"
)

func UploadRpmPackage(ctx *context.Context, upload io.Reader) (int, *packages_model.PackageVersion, error) {
	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	defer buf.Close()

	pck, err := rpm_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			return http.StatusBadRequest, nil, err
		}

		return http.StatusInternalServerError, nil, err
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	fileMetadataRaw, err := json.Marshal(pck.FileMetadata)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeRpm,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:  ctx.Doer,
			Metadata: pck.VersionMetadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: fmt.Sprintf("%s-%s.%s.rpm", pck.Name, pck.Version, pck.FileMetadata.Architecture),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				rpm_module.PropertyMetadata: string(fileMetadataRaw),
			},
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion, packages_model.ErrDuplicatePackageFile:
			return http.StatusConflict, nil, err
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			return http.StatusForbidden, nil, err
		default:
			return http.StatusInternalServerError, nil, err
		}
	}

	if err := rpm_service.BuildRepositoryFiles(ctx, ctx.Package.Owner.ID); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, pv, nil
}
