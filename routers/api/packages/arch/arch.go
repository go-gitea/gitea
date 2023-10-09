// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
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

	buf, err := pkg_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	desc, err := arch_module.EjectMetadata(filename, distro, buf)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	properties := map[string]string{
		"desc": desc.String(),
	}
	if sign != "" {
		_, err := hex.DecodeString(sign)
		if err != nil {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		properties["sign"] = sign
	}

	_, _, err = pkg_service.CreatePackageOrAddFileToExisting(
		ctx, &pkg_service.PackageCreationInfo{
			PackageInfo: pkg_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: pkg_model.TypeArch,
				Name:        desc.Name,
				Version:     desc.Version,
			},
			Creator: ctx.Doer,
			Metadata: &arch_module.Metadata{
				URL:          desc.ProjectURL,
				Description:  desc.Description,
				Provides:     desc.Provides,
				License:      desc.License,
				Depends:      desc.Depends,
				OptDepends:   desc.OptDepends,
				MakeDepends:  desc.MakeDepends,
				CheckDepends: desc.CheckDepends,
			},
		},
		&pkg_service.PackageFileCreationInfo{
			PackageFileInfo: pkg_service.PackageFileInfo{
				Filename:     filename,
				CompositeKey: distro,
			},
			OverwriteExisting: true,
			IsLead:            true,
			Creator:           ctx.ContextUser,
			Data:              buf,
			Properties:        properties,
		},
	)
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

	version, err := pkg_model.GetVersionByNameAndVersion(
		ctx, ctx.Package.Owner.ID, pkg_model.TypeArch, pkg, ver,
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	err = pkg_service.RemovePackageVersion(ctx, ctx.Package.Owner, version)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}
