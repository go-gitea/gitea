// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
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

func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := arch_service.GetOrCreateKeyPair(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.ServeContent(strings.NewReader(pub), &context.ServeHeaderOptions{
		ContentType: "application/pgp-keys",
		Filename:    "repository.key",
	})
}

func PushPackage(ctx *context.Context) {
	distro := ctx.PathParam("distro")

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

	p, err := arch_module.ParsePackage(buf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	sign, err := arch_service.NewFileSign(ctx, ctx.Package.Owner.ID, buf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer sign.Close()
	_, err = buf.Seek(0, io.SeekStart)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	// update gpg sign
	pgp, err := io.ReadAll(sign)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	p.FileMetadata.PgpSigned = base64.StdEncoding.EncodeToString(pgp)
	_, err = sign.Seek(0, io.SeekStart)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	properties := map[string]string{
		arch_module.PropertyDescription:  p.Desc(),
		arch_module.PropertyArch:         p.FileMetadata.Arch,
		arch_module.PropertyDistribution: distro,
	}

	version, _, err := packages_service.CreatePackageOrAddFileToExisting(
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
			OverwriteExisting: false,
			IsLead:            true,
			Creator:           ctx.ContextUser,
			Data:              buf,
			Properties:        properties,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, packages_model.ErrDuplicatePackageVersion), errors.Is(err, packages_model.ErrDuplicatePackageFile):
			apiError(ctx, http.StatusConflict, err)
		case errors.Is(err, packages_service.ErrQuotaTotalCount), errors.Is(err, packages_service.ErrQuotaTypeSize), errors.Is(err, packages_service.ErrQuotaTotalSize):
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	// add sign file
	_, err = packages_service.AddFileToPackageVersionInternal(ctx, version, &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{
			CompositeKey: distro,
			Filename:     fmt.Sprintf("%s-%s-%s.pkg.tar.zst.sig", p.Name, p.Version, p.FileMetadata.Arch),
		},
		OverwriteExisting: true,
		IsLead:            false,
		Creator:           ctx.Doer,
		Data:              sign,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	if err = arch_service.BuildPacmanDB(ctx, ctx.Package.Owner.ID, distro, p.FileMetadata.Arch); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Status(http.StatusCreated)
}

func GetPackageOrDB(ctx *context.Context) {
	var (
		file   = ctx.PathParam("file")
		distro = ctx.PathParam("distro")
		arch   = ctx.PathParam("arch")
	)

	if strings.HasSuffix(file, ".pkg.tar.zst") || strings.HasSuffix(file, ".pkg.tar.zst.sig") {
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

	if strings.HasSuffix(file, ".db.tar.gz") ||
		strings.HasSuffix(file, ".db") ||
		strings.HasSuffix(file, ".db.tar.gz.sig") ||
		strings.HasSuffix(file, ".db.sig") {
		pkg, err := arch_service.GetPackageDBFile(ctx, distro, arch, ctx.Package.Owner.ID,
			strings.HasSuffix(file, ".sig"))
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

	ctx.Status(http.StatusNotFound)
}

func RemovePackage(ctx *context.Context) {
	var (
		distro = ctx.PathParam("distro")
		pkg    = ctx.PathParam("package")
		ver    = ctx.PathParam("version")
	)
	pv, err := packages_model.GetVersionByNameAndVersion(
		ctx, ctx.Package.Owner.ID, packages_model.TypeArch, pkg, ver,
	)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	files, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	deleted := false
	for _, file := range files {
		if file.CompositeKey == distro {
			deleted = true
			err := packages_service.RemovePackageFileAndVersionIfUnreferenced(ctx, ctx.ContextUser, file)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
		}
	}
	if deleted {
		err = arch_service.BuildCustomRepositoryFiles(ctx, ctx.Package.Owner.ID, distro)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.Error(http.StatusNotFound)
	}
}
