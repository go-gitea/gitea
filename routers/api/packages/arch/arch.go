// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	pkg_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	pkg_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/routers/api/packages/helper"
	pkg_service "code.gitea.io/gitea/services/packages"
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
	desc, err := arch_module.EjectMetadata(&arch_module.EjectParams{
		Filename:     filename,
		Distribution: distro,
		Buffer:       buf,
	})
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Metadata related to SQL database.
	md := &arch_module.Metadata{
		URL:          desc.URL,
		Description:  desc.Description,
		Provides:     desc.Provides,
		License:      desc.License,
		Depends:      desc.Depends,
		OptDepends:   desc.OptDepends,
		MakeDepends:  desc.MakeDepends,
		CheckDepends: desc.CheckDepends,
		Backup:       desc.Backup,
		DistroArch:   []string{distro + "-" + desc.Arch[0]},
	}

	// Save file related to arch package.
	pkgid, err := arch_service.SaveFile(ctx, &arch_service.SaveFileParams{
		Creator:  ctx.ContextUser,
		Owner:    ctx.Package.Owner,
		Metadata: md,
		Buf:      buf,
		Filename: filename,
		Distro:   distro,
		PkgName:  desc.Name,
		PkgVer:   desc.Version,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	r := io.NopCloser(bytes.NewReader([]byte(desc.GetDbDesc())))
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
		Filename: desc.Name + "-" + desc.Version + "-" + desc.Arch[0] + ".desc",
		Buf:      buf,
		Metadata: md,
		Distro:   distro,
		PkgName:  desc.Name,
		PkgVer:   desc.Version,
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
			Metadata: md,
			Distro:   distro,
			PkgName:  desc.Name,
			PkgVer:   desc.Version,
		})
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	// Add new architectures and distribution info to package version metadata.
	err = arch_service.UpdateMetadata(ctx, &arch_service.UpdateMetadataParameters{
		User:     ctx.Package.Owner,
		Metadata: md,
		DbDesc:   desc,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Automatically connect repository with souce code if name matched.
	err = arch_service.RepoConnect(ctx, owner, desc.Name, pkgid)
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

	// Packages and signatures are loaded directly from object storage.
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

	// Pacman databases is not stored in giteas storage and created 'on-request'
	// for user/organization scope with accordance to requested architecture
	// and distribution.
	if strings.HasSuffix(file, ".db.tar.gz") || strings.HasSuffix(file, ".db") {
		db, err := arch_service.CreatePacmanDb(ctx, &arch_service.PacmanDbParams{
			Owner:        owner,
			Architecture: arch,
			Distribution: distro,
		})
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

	version, err := pkg_model.GetVersionByNameAndVersion(
		ctx, ctx.Package.Owner.ID, pkg_model.TypeArch, pkg, ver,
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	err = pkg_service.RemovePackageVersion(ctx.Package.Owner, version)
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
