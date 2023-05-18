// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	alpine_module "code.gitea.io/gitea/modules/packages/alpine"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
	alpine_service "code.gitea.io/gitea/services/packages/alpine"
)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := alpine_service.GetOrCreateKeyPair(ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pubPem, _ := pem.Decode([]byte(pub))
	if pubPem == nil {
		apiError(ctx, http.StatusInternalServerError, "failed to decode private key pem")
		return
	}

	pubKey, err := x509.ParsePKIXPublicKey(pubPem.Bytes)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	fingerprint, err := util.CreatePublicKeyFingerprint(pubKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.ServeContent(strings.NewReader(pub), &context.ServeHeaderOptions{
		ContentType: "application/x-pem-file",
		Filename:    fmt.Sprintf("%s@%s.rsa.pub", ctx.Package.Owner.LowerName, hex.EncodeToString(fingerprint)),
	})
}

func GetRepositoryFile(ctx *context.Context) {
	pv, err := alpine_service.GetOrCreateRepositoryVersion(ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	s, pf, err := packages_service.GetFileStreamByPackageVersion(
		ctx,
		pv,
		&packages_service.PackageFileInfo{
			Filename:     alpine_service.IndexFilename,
			CompositeKey: fmt.Sprintf("%s|%s|%s", ctx.Params("branch"), ctx.Params("repository"), ctx.Params("architecture")),
		},
	)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

func UploadPackageFile(ctx *context.Context) {
	branch := strings.TrimSpace(ctx.Params("branch"))
	repository := strings.TrimSpace(ctx.Params("repository"))
	if branch == "" || repository == "" {
		apiError(ctx, http.StatusBadRequest, "invalid branch or repository")
		return
	}

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pck, err := alpine_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) || err == io.EOF {
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

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
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
		case packages_model.ErrDuplicatePackageVersion, packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusBadRequest, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := alpine_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, branch, repository, pck.FileMetadata.Architecture); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

func DownloadPackageFile(ctx *context.Context) {
	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:      ctx.Package.Owner.ID,
		PackageType:  packages_model.TypeAlpine,
		Query:        ctx.Params("filename"),
		CompositeKey: fmt.Sprintf("%s|%s|%s", ctx.Params("branch"), ctx.Params("repository"), ctx.Params("architecture")),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	s, pf, err := packages_service.GetPackageFileStream(ctx, pfs[0])
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

func DeletePackageFile(ctx *context.Context) {
	branch, repository, architecture := ctx.Params("branch"), ctx.Params("repository"), ctx.Params("architecture")

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:      ctx.Package.Owner.ID,
		PackageType:  packages_model.TypeAlpine,
		Query:        ctx.Params("filename"),
		CompositeKey: fmt.Sprintf("%s|%s|%s", branch, repository, architecture),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	if err := packages_service.RemovePackageFileAndVersionIfUnreferenced(ctx.Doer, pfs[0]); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := alpine_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, branch, repository, architecture); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
