// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
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

	desc, err := arch_module.EjectMetadata(&arch_module.EjectParams{
		Filename:     filename,
		Distribution: distro,
		Buffer:       buf,
	})
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

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

	ver, _, err := pkg_service.CreatePackageOrAddFileToExisting(
		&pkg_service.PackageCreationInfo{
			PackageInfo: pkg_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: pkg_model.TypeArch,
				Name:        desc.Name,
				Version:     desc.Version,
			},
			Creator:  ctx.ContextUser,
			Metadata: md,
		},
		&pkg_service.PackageFileCreationInfo{
			PackageFileInfo: pkg_service.PackageFileInfo{
				Filename:     filename,
				CompositeKey: distro + "-" + filename,
			},
			OverwriteExisting: true,
			IsLead:            true,
			Creator:           ctx.ContextUser,
			Data:              buf,
		},
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = pkg_model.InsertProperty(ctx, 0, ver.ID, distro+"-"+filename+".desc", desc.String())
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = pkg_model.InsertProperty(ctx, 0, ver.ID, distro+"-"+filename+".sig", sign)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	err = arch_service.UpdateMetadata(ctx, &arch_service.UpdateMetadataParams{
		User:     ctx.Package.Owner,
		Metadata: md,
		DbDesc:   desc,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	err = arch_service.RepoConnect(ctx, owner, desc.Name, ver.ID)
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

	// Packages are loaded directly from object storage.
	if strings.HasSuffix(file, ".pkg.tar.zst") {
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

	// Signatures are loaded from package properties in SQL db.
	if strings.HasSuffix(file, ".pkg.tar.zst.sig") {
		sign, err := arch_service.GetProperty(ctx, owner, distro+"-"+file)
		if err != nil {
			apiError(ctx, http.StatusNotFound, err)
			return
		}

		ctx.ServeContent(bytes.NewReader(sign), &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	// Pacman databases is not stored in gitea storage and created 'on-request'
	// for user/organization scope with accordance to requested architecture
	// and distribution.
	if strings.HasSuffix(file, ".db.tar.gz") || strings.HasSuffix(file, ".db") {
		db, err := arch_service.CreatePacmanDb(ctx, &arch_service.DbParams{
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
