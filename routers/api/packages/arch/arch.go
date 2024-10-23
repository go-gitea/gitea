// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/globallock"
	packages_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
	arch_service "code.gitea.io/gitea/services/packages/arch"
)

var (
	archPkgOrSig = regexp.MustCompile(`^.*\.pkg\.tar\.\w+(\.sig)*$`)
	archDBOrSig  = regexp.MustCompile(`^.*.db(\.tar\.gz)*(\.sig)*$`)
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

func refreshLocker(ctx *context.Context, group string) (globallock.ReleaseFunc, error) {
	return globallock.Lock(ctx, fmt.Sprintf("pkg_%d_arch_pkg_%s", ctx.Package.Owner.ID, group))
}

func PushPackage(ctx *context.Context) {
	group := ctx.PathParam("group")
	releaser, err := refreshLocker(ctx, group)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer releaser()
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
		apiError(ctx, http.StatusBadRequest, err)
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
		arch_module.PropertyDistribution: group,
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
				Filename:     fmt.Sprintf("%s-%s-%s.pkg.tar.%s", p.Name, p.Version, p.FileMetadata.Arch, p.CompressType),
				CompositeKey: group,
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
			CompositeKey: group,
			Filename:     fmt.Sprintf("%s-%s-%s.pkg.tar.%s.sig", p.Name, p.Version, p.FileMetadata.Arch, p.CompressType),
		},
		OverwriteExisting: true,
		IsLead:            false,
		Creator:           ctx.Doer,
		Data:              sign,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err = arch_service.BuildPacmanDB(ctx, ctx.Package.Owner.ID, group, p.FileMetadata.Arch); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Status(http.StatusCreated)
}

func GetPackageOrDB(ctx *context.Context) {
	var (
		file  = ctx.PathParam("file")
		group = ctx.PathParam("group")
		arch  = ctx.PathParam("arch")
	)

	if archPkgOrSig.MatchString(file) {
		pkg, u, pf, err := arch_service.GetPackageFile(ctx, group, file, ctx.Package.Owner.ID)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
		helper.ServePackageFile(ctx, pkg, u, pf)
		return
	}

	if archDBOrSig.MatchString(file) {
		pkg, u, pf, err := arch_service.GetPackageDBFile(ctx, group, arch, ctx.Package.Owner.ID,
			strings.HasSuffix(file, ".sig"))
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}

		helper.ServePackageFile(ctx, pkg, u, pf)
		return
	}

	ctx.Status(http.StatusNotFound)
}

func RemovePackage(ctx *context.Context) {
	var (
		group   = ctx.PathParam("group")
		pkg     = ctx.PathParam("package")
		ver     = ctx.PathParam("version")
		pkgArch = ctx.PathParam("arch")
	)
	releaser, err := refreshLocker(ctx, group)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer releaser()
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
		extName := fmt.Sprintf("-%s.pkg.tar%s", pkgArch, filepath.Ext(file.LowerName))
		if strings.HasSuffix(file.LowerName, ".sig") {
			extName = fmt.Sprintf("-%s.pkg.tar%s.sig", pkgArch,
				filepath.Ext(strings.TrimSuffix(file.LowerName, filepath.Ext(file.LowerName))))
		}
		if file.CompositeKey == group &&
			strings.HasSuffix(file.LowerName, extName) {
			deleted = true
			err := packages_service.RemovePackageFileAndVersionIfUnreferenced(ctx, ctx.ContextUser, file)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
		}
	}
	if deleted {
		err = arch_service.BuildCustomRepositoryFiles(ctx, ctx.Package.Owner.ID, group)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	} else {
		ctx.Error(http.StatusNotFound)
	}
}
