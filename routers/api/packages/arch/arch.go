// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

// Push new package to arch package registry.
func Push(ctx *context.Context) {
	var (
		filename = ctx.Params("filename")
		distro   = ctx.Params("distro")
		sign     = ctx.Params("sign")
	)

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if close {
		defer upload.Close()
	}

	_, _, err = arch_service.UploadArchPackage(ctx, upload, filename, distro, sign)
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

	ctx.Status(http.StatusOK)
}

// Get file from arch package registry.
func Get(ctx *context.Context) {
	var (
		file   = ctx.Params("file")
		owner  = ctx.Params("username")
		distro = ctx.Params("distro")
		arch   = ctx.Params("arch")
	)

	if strings.HasSuffix(file, ".pkg.tar.zst") {
		pkg, err := arch_service.GetPackageFile(ctx, distro, file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		ctx.ServeContent(pkg, &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	if strings.HasSuffix(file, ".pkg.tar.zst.sig") {
		sig, err := arch_service.GetPackageSignature(ctx, distro, file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		ctx.ServeContent(sig, &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	if strings.HasSuffix(file, ".db.tar.gz") || strings.HasSuffix(file, ".db") {
		db, err := arch_service.CreatePacmanDb(ctx, owner, arch, distro)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		ctx.ServeContent(db, &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	ctx.Status(http.StatusNotFound)
}

// Remove specific package version, related files with properties.
func Remove(ctx *context.Context) {
	var (
		pkg = ctx.Params("package")
		ver = ctx.Params("version")
	)

	version, err := packages_model.GetVersionByNameAndVersion(
		ctx, ctx.Package.Owner.ID, packages_model.TypeArch, pkg, ver,
	)
	if err != nil {
		switch err {
		case packages_model.ErrPackageNotExist:
			apiError(ctx, http.StatusNotFound, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	err = packages_service.RemovePackageVersion(ctx, ctx.Package.Owner, version)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}
