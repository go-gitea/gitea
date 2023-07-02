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
		distro   = ctx.Req.Header.Get("distro")
		sendtime = ctx.Req.Header.Get("time")
		pkgsign  = ctx.Req.Header.Get("pkgsign")
		metasign = ctx.Req.Header.Get("metasign")
	)

	// Decoding package signature.
	sigdata, err := hex.DecodeString(pkgsign)
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

	// Decoding time when message was created.
	t, err := time.Parse(time.RFC3339, sendtime)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if time.Since(t) > time.Hour {
		apiError(ctx, http.StatusUnauthorized, "outdated message")
		return
	}

	// Decoding signature related to metadata.
	msigdata, err := hex.DecodeString(metasign)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Validating metadata signature, to ensure that operation push operation
	// is initiated by original package owner.
	sendmetadata := []byte(owner + filename + sendtime)
	err = arch_service.ValidateSignature(ctx, sendmetadata, msigdata, user)
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, err)
		return
	}

	// Validate package signature with any of user's GnuPG keys.
	err = arch_service.ValidateSignature(ctx, pkgdata, sigdata, user)
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, err)
		return
	}

	// Parse metadata contained in arch package archive.
	md, err := arch_module.EjectMetadata(filename, distro, setting.Domain, pkgdata)
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
		apiError(ctx, http.StatusInternalServerError, err)
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
		pkgdata, err := arch_service.LoadFile(ctx, distro, file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		ctx.ServeContent(bytes.NewReader(pkgdata), &context.ServeHeaderOptions{
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
		owner   = ctx.Params("username")
		email   = ctx.Req.Header.Get("email")
		target  = ctx.Req.Header.Get("target")
		stime   = ctx.Req.Header.Get("time")
		version = ctx.Req.Header.Get("version")
	)

	// Parse sent time and check if it is within last minute.
	t, err := time.Parse(time.RFC3339, stime)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if time.Since(t) > time.Minute {
		apiError(ctx, http.StatusUnauthorized, "outdated message")
		return
	}

	// Get user owning the package.
	user, org, err := arch_service.IdentifyOwner(ctx, owner, email)
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, err)
		return
	}

	// Read signature data from request body.
	sigdata, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer ctx.Req.Body.Close()

	// Validate package signature with any of user's GnuPG keys.
	mesdata := []byte(owner + target + stime)
	err = arch_service.ValidateSignature(ctx, mesdata, sigdata, user)
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, err)
		return
	}

	// Remove package files and pacman database entry.
	err = arch_service.RemovePackage(ctx, org.AsUser(), target, version)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Resp.WriteHeader(http.StatusOK)
}

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}
