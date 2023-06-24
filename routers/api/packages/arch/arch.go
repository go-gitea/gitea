// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

// Push new package to arch package registry.
func Push(ctx *context.Context) {
	var (
		owner    = ctx.Params("username")
		filename = ctx.Req.Header.Get("filename")
		email    = ctx.Req.Header.Get("email")
		sign     = ctx.Req.Header.Get("sign")
		distro   = ctx.Req.Header.Get("distro")
	)

	// Decoding package signature.
	sigdata, err := hex.DecodeString(sign)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Read package to memory for signature validation.
	pkgdata, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer ctx.Req.Body.Close()

	// Get user and organization owning arch package.
	user, org, err := arch_service.IdentifyOwner(ctx, owner, email)
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, err)
		return
	}

	// Validate package signature with any of user's GnuPG keys.
	err = arch_service.ValidatePackageSignature(ctx, pkgdata, sigdata, user)
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, err)
		return
	}

	// Parse metadata contained in arch package archive.
	md, err := arch_module.EjectMetadata(filename, setting.Domain, pkgdata)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Save file related to arch package.
	pkgid, err := arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
		Organization: org,
		User:         user,
		Metadata:     md,
		Filename:     filename,
		Data:         pkgdata,
		Distro:       distro,
	})
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Save file related to arch package signature.
	_, err = arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
		Organization: org,
		User:         user,
		Metadata:     md,
		Data:         sigdata,
		Filename:     filename + ".sig",
		Distro:       distro,
	})
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Automatically connect repository for provided package if name matched.
	err = arch_service.RepositoryAutoconnect(ctx, owner, md.Name, pkgid)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Update pacman databases with new package.
	err = arch_service.UpdatePacmanDatabases(ctx, md, distro, owner)
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
		pkgdata, err := arch_service.LoadPackageFile(ctx, distro, file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		_, err = ctx.Resp.Write(pkgdata)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		ctx.Resp.WriteHeader(http.StatusOK)
		return
	}

	// Pacman databases are stored directly in gitea file storage and could be
	// loaded with name as a key.
	if strings.HasSuffix(file, ".db.tar.gz") || strings.HasSuffix(file, ".db") {
		data, err := arch_service.LoadPacmanDatabase(ctx, owner, distro, arch, file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		_, err = ctx.Resp.Write(data)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		ctx.Resp.WriteHeader(http.StatusOK)
		return
	}

	ctx.Resp.WriteHeader(http.StatusNotFound)
}

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}
