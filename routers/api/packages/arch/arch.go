// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := arch_service.GetOrCreateKeyPair(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.ServeContent(strings.NewReader(pub), &context.ServeHeaderOptions{
		ContentType: "application/pgp-keys",
	})
}

func UploadPackageFile(ctx *context.Context) {
	repository := strings.TrimSpace(ctx.PathParam("repository"))

	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if needToClose {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pck, err := arch_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) || errors.Is(err, io.EOF) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	fileMetadataRaw, err := json.Marshal(pck.FileMetadata)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	signature, err := arch_service.SignData(ctx, ctx.Package.Owner.ID, buf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	release, err := arch_service.AquireRegistryLock(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	// Search for duplicates with different file compression
	has, err := packages_model.HasFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:     ctx.Package.Owner.ID,
		PackageType: packages_model.TypeArch,
		Query:       fmt.Sprintf("%s-%s-%s.pkg.tar.%%", pck.Name, pck.Version, pck.FileMetadata.Architecture),
		Properties: map[string]string{
			arch_module.PropertyRepository:   repository,
			arch_module.PropertyArchitecture: pck.FileMetadata.Architecture,
		},
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if has {
		apiError(ctx, http.StatusConflict, packages_model.ErrDuplicatePackageFile)
		return
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeArch,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:  ctx.Doer,
			Metadata: pck.VersionMetadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s-%s-%s.pkg.tar.%s", pck.Name, pck.Version, pck.FileMetadata.Architecture, pck.FileCompressionExtension),
				CompositeKey: fmt.Sprintf("%s|%s", repository, pck.FileMetadata.Architecture),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				arch_module.PropertyRepository:   repository,
				arch_module.PropertyArchitecture: pck.FileMetadata.Architecture,
				arch_module.PropertyMetadata:     string(fileMetadataRaw),
				arch_module.PropertySignature:    base64.StdEncoding.EncodeToString(signature),
			},
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion, packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := arch_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, repository, pck.FileMetadata.Architecture); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

func GetPackageOrRepositoryFile(ctx *context.Context) {
	repository := ctx.PathParam("repository")
	architecture := ctx.PathParam("architecture")
	filename := ctx.PathParam("filename")
	filenameOrig := filename

	isSignature := strings.HasSuffix(filename, ".sig")
	if isSignature {
		filename = filename[:len(filename)-len(".sig")]
	}

	opts := &packages_model.PackageFileSearchOptions{
		OwnerID:      ctx.Package.Owner.ID,
		PackageType:  packages_model.TypeArch,
		Query:        filename,
		CompositeKey: fmt.Sprintf("%s|%s", repository, architecture),
	}

	if strings.HasSuffix(filename, ".db.tar.gz") || strings.HasSuffix(filename, ".files.tar.gz") || strings.HasSuffix(filename, ".files") || strings.HasSuffix(filename, ".db") {
		// The requested filename is based on the user-defined repository name.
		// Normalize everything to "packages.db".
		opts.Query = arch_service.IndexArchiveFilename

		pv, err := arch_service.GetOrCreateRepositoryVersion(ctx, ctx.Package.Owner.ID)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		opts.VersionID = pv.ID
	}

	pfs, _, err := packages_model.SearchFiles(ctx, opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) == 0 {
		// Try again with architecture 'any'
		if architecture == arch_module.AnyArch {
			apiError(ctx, http.StatusNotFound, nil)
			return
		}

		opts.CompositeKey = fmt.Sprintf("%s|%s", repository, arch_module.AnyArch)
		if pfs, _, err = packages_model.SearchFiles(ctx, opts); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	if isSignature {
		pfps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeFile, pfs[0].ID, arch_module.PropertySignature)
		if err != nil || len(pfps) == 0 {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		data, err := base64.StdEncoding.DecodeString(pfps[0].Value)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		ctx.ServeContent(bytes.NewReader(data), &context.ServeHeaderOptions{
			Filename: filenameOrig,
		})
		return
	}

	s, u, pf, err := packages_service.GetPackageFileStream(ctx, pfs[0])
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

func DeletePackageVersion(ctx *context.Context) {
	repository := ctx.PathParam("repository")
	architecture := ctx.PathParam("architecture")
	name := ctx.PathParam("name")
	version := ctx.PathParam("version")

	release, err := arch_service.AquireRegistryLock(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeArch, name, version)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID:    pv.ID,
		CompositeKey: fmt.Sprintf("%s|%s", repository, architecture),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	if err := packages_service.RemovePackageFileAndVersionIfUnreferenced(ctx, ctx.Doer, pfs[0]); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := arch_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, repository, architecture); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
