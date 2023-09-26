// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
	alpine_service "code.gitea.io/gitea/services/packages/alpine"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := alpine_service.GetOrCreateKeyPair(ctx, ctx.Package.Owner.ID)
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
	pv, err := alpine_service.GetOrCreateRepositoryVersion(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	s, u, pf, err := packages_service.GetFileStreamByPackageVersion(
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

	helper.ServePackageFile(ctx, s, u, pf)
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

	statusCode, _, err := alpine_service.UploadAlpinePackage(ctx, upload, branch, repository)
	if err != nil {
		apiError(ctx, statusCode, err)
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

	if err := packages_service.RemovePackageFileAndVersionIfUnreferenced(ctx, ctx.Doer, pfs[0]); err != nil {
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
