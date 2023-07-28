// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	pkg_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/routers/api/packages/helper"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

// Push new package to arch package registry.
func Push(ctx *context.Context) {
	var (
		owner    = ctx.Params("username")
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

	buf, err := pkg_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	// Parse metadata related to package contained in arch package archive.
	pkgmd, err := arch_module.EjectMetadata(filename, distro, buf)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Metadata related to SQL database.
	dbmd := &arch_module.Metadata{
		Name:         pkgmd.Name,
		Version:      pkgmd.Version,
		URL:          pkgmd.URL,
		Description:  pkgmd.Description,
		Provides:     pkgmd.Provides,
		License:      pkgmd.License,
		Depends:      pkgmd.Depends,
		OptDepends:   pkgmd.OptDepends,
		MakeDepends:  pkgmd.MakeDepends,
		CheckDepends: pkgmd.CheckDepends,
		Backup:       pkgmd.Backup,
		DistroArch:   []string{distro + "-" + pkgmd.Arch[0]},
	}

	// Save file related to arch package.
	pkgid, err := arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
		Creator:  ctx.ContextUser,
		Owner:    ctx.Package.Owner,
		Filename: filename,
		Buf:      buf,
		Metadata: dbmd,
		Distro:   distro,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	r := io.NopCloser(bytes.NewReader([]byte(pkgmd.GetDbDesc())))
	buf, err = pkg_module.CreateHashedBufferFromReader(r)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	// Save file related to arch package description for pacman database.
	_, err = arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
		Creator:  ctx.ContextUser,
		Owner:    ctx.Package.Owner,
		Filename: pkgmd.Name + "-" + pkgmd.Version + "-" + pkgmd.Arch[0] + ".desc",
		Buf:      buf,
		Metadata: dbmd,
		Distro:   distro,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Decoding package signature, if present saving with package as file.
	sigdata, err := hex.DecodeString(sign)
	if err == nil {
		r := io.NopCloser(bytes.NewReader(sigdata))
		buf, err := pkg_module.CreateHashedBufferFromReader(r)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		defer buf.Close()

		_, err = arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
			Creator:  ctx.ContextUser,
			Owner:    ctx.Package.Owner,
			Buf:      buf,
			Filename: filename + ".sig",
			Metadata: dbmd,
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
		Md:   dbmd,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Automatically connect repository for provided package if name matched.
	err = arch_service.RepositoryAutoconnect(ctx, owner, dbmd.Name, pkgid)
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
			Filename: file,
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
			Filename: file,
		})
		return
	}

	ctx.Status(http.StatusNotFound)
}

// Remove specific package version, related files and pacman database entry.
func Remove(ctx *context.Context) {
	var (
		pkg = ctx.Params("package")
		ver = ctx.Params("version")
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
