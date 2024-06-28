// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/base"
	packages_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func Push(ctx *context.Context) {
	var (
		distro = ctx.PathParam("distro")
		sign   = ctx.PathParam("sign")
	)

	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if needToClose {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	_, _, sha256, _ := buf.Sums()

	p, err := arch_module.ParsePackage(buf, sha256, buf.Size())
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	properties := map[string]string{
		arch_module.PropertyDescription:    p.Desc(),
		arch_module.PropertyCompressedSize: base.FileSize(p.FileMetadata.CompressedSize),
		arch_module.PropertyInstalledSize:  base.FileSize(p.FileMetadata.InstalledSize),
		arch_module.PropertySHA256:         p.FileMetadata.SHA256,
		arch_module.PropertyBuildDate:      time.Unix(p.FileMetadata.BuildDate, 0).Format(time.RFC3339),
		arch_module.PropertyPackager:       p.FileMetadata.Packager,
		arch_module.PropertyArch:           p.FileMetadata.Arch,
		arch_module.PropertyDistribution:   distro,
	}
	if sign != "" {
		_, err := base64.RawURLEncoding.DecodeString(sign)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		properties[arch_module.PropertySignature] = sign
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeArch,
				Name:        p.Name,
				Version:     p.Version,
			},
			Creator:  ctx.Doer,
			Metadata: p.VersionMetadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s-%s-%s.pkg.tar.zst", p.Name, p.Version, p.FileMetadata.Arch),
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

func Get(ctx *context.Context) {
	var (
		file   = ctx.PathParam("file")
		owner  = ctx.PathParam("username")
		distro = ctx.PathParam("distro")
		arch   = ctx.PathParam("arch")
	)

	if strings.HasSuffix(file, ".pkg.tar.zst") {
		pkg, err := arch_service.GetPackageFile(ctx, distro, file, ctx.Package.Owner.ID)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}

		ctx.ServeContent(pkg, &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	if strings.HasSuffix(file, ".pkg.tar.zst.sig") {
		sig, err := arch_service.GetPackageSignature(ctx, distro, file, ctx.Package.Owner.ID)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}

		ctx.ServeContent(sig, &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	if strings.HasSuffix(file, ".db.tar.gz") || strings.HasSuffix(file, ".db") {
		db, err := arch_service.CreatePacmanDb(ctx, owner, arch, distro, ctx.Package.Owner.ID)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		ctx.ServeContent(bytes.NewReader(db.Bytes()), &context.ServeHeaderOptions{
			Filename: file,
		})
		return
	}

	ctx.Status(http.StatusNotFound)
}

func Remove(ctx *context.Context) {
	var (
		pkg    = ctx.PathParam("package")
		ver    = ctx.PathParam("version")
		distro = ctx.PathParam("distro")
		arch   = ctx.PathParam("arch")
	)

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:      ctx.Package.Owner.ID,
		PackageType:  packages_model.TypeArch,
		Query:        fmt.Sprintf("%s-%s-%s.pkg.tar.zst", pkg, arch, ver),
		CompositeKey: distro,
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

	ctx.Status(http.StatusNoContent)
}
