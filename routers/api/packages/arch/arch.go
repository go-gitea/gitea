// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/context"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/routers/api/packages/helper"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

// Push new package to arch package registry.
func Push(ctx *context.Context) {
	var (
		owner    = ctx.Params("username")
		filename = ctx.Req.Header.Get("filename")
		distro   = ctx.Req.Header.Get("distro")
		sign     = ctx.Req.Header.Get("sign")
	)

	// Read package to memory for package validation.
	pkgdata, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer ctx.Req.Body.Close()

	// Parse metadata contained in arch package archive.
	md, err := arch_module.EjectMetadata(filename, distro, pkgdata)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Save file related to arch package.
	pkgid, err := arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
		Creator:  ctx.ContextUser,
		Owner:    ctx.Package.Owner,
		Metadata: md,
		Filename: filename,
		Data:     pkgdata,
		Distro:   distro,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Decoding package signature, if present saving with package as file.
	sigdata, err := hex.DecodeString(sign)
	if err == nil {
		_, err = arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
			Creator:  ctx.ContextUser,
			Owner:    ctx.Package.Owner,
			Metadata: md,
			Data:     sigdata,
			Filename: filename + ".sig",
			Distro:   distro,
		})
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	// Add new architectures and distribution info to package version metadata.
	err = arch_service.UpdateMetadata(ctx, &arch_service.UpdateMetadataParameters{
		User: ctx.Package.Owner,
		Md:   md,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Automatically connect repository for provided package if name matched.
	err = arch_service.RepositoryAutoconnect(ctx, owner, md.Name, pkgid)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
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

	// Packages are stored in different way from pacman databases, and loaded
	// with LoadPackageFile function.
	if strings.HasSuffix(file, "tar.zst") || strings.HasSuffix(file, "zst.sig") {
		pkg, err := arch_service.GetFileObject(ctx, distro, file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		ctx.ServeContent(pkg, &context.ServeHeaderOptions{
			Filename:      file,
			CacheDuration: time.Minute * 5,
		})
		return
	}

	// Pacman databases is not stored in gitea's storage, it is created for
	// incoming request and cached.
	if strings.HasSuffix(file, ".db.tar.gz") || strings.HasSuffix(file, ".db") {
		db, err := arch_service.CreatePacmanDb(ctx, owner, arch, distro)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		ctx.ServeContent(bytes.NewReader(db), &context.ServeHeaderOptions{
			Filename:      file,
			CacheDuration: time.Minute * 5,
		})
		return
	}

	ctx.Resp.WriteHeader(http.StatusNotFound)
}

// Remove specific package version, related files and pacman database entry.
func Remove(ctx *context.Context) {
	var (
		pkg = ctx.Req.Header.Get("package")
		ver = ctx.Req.Header.Get("version")
	)

	// Remove package files and pacman database entry.
	err := arch_service.RemovePackage(ctx, ctx.Package.Owner, pkg, ver)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}
