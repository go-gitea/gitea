// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	stdctx "context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/notification"
	packages_module "code.gitea.io/gitea/modules/packages"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
	debian_service "code.gitea.io/gitea/services/packages/debian"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := debian_service.GetOrCreateKeyPair(ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.ServeContent(strings.NewReader(pub), &context.ServeHeaderOptions{
		ContentType: "application/pgp-keys",
		Filename:    "repository.key",
	})
}

// https://wiki.debian.org/DebianRepository/Format#A.22Release.22_files
// https://wiki.debian.org/DebianRepository/Format#A.22Packages.22_Indices
func GetRepositoryFile(ctx *context.Context) {
	pv, err := debian_service.GetOrCreateRepositoryVersion(ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	key := ctx.Params("distribution")

	component := ctx.Params("component")
	architecture := strings.TrimPrefix(ctx.Params("architecture"), "binary-")
	if component != "" && architecture != "" {
		key += "|" + component + "|" + architecture
	}

	s, pf, err := packages_service.GetFileStreamByPackageVersion(
		ctx,
		pv,
		&packages_service.PackageFileInfo{
			Filename:     ctx.Params("filename"),
			CompositeKey: key,
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

// https://wiki.debian.org/DebianRepository/Format#indices_acquisition_via_hashsums_.28by-hash.29
func GetRepositoryFileByHash(ctx *context.Context) {
	pv, err := debian_service.GetOrCreateRepositoryVersion(ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	algorithm := strings.ToLower(ctx.Params("algorithm"))
	if algorithm == "md5sum" {
		algorithm = "md5"
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID:     pv.ID,
		Hash:          strings.ToLower(ctx.Params("hash")),
		HashAlgorithm: algorithm,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	s, pf, err := packages_service.GetPackageFileStream(ctx, pfs[0])
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

func UploadPackageFile(ctx *context.Context) {
	distribution := strings.TrimSpace(ctx.Params("distribution"))
	component := strings.TrimSpace(ctx.Params("component"))
	if distribution == "" || component == "" {
		apiError(ctx, http.StatusBadRequest, "invalid distribution or component")
		return
	}

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pck, err := debian_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeDebian,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:  ctx.Doer,
			Metadata: pck.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s_%s_%s.deb", pck.Name, pck.Version, pck.Architecture),
				CompositeKey: fmt.Sprintf("%s|%s", distribution, component),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				debian_module.PropertyDistribution: distribution,
				debian_module.PropertyComponent:    component,
				debian_module.PropertyArchitecture: pck.Architecture,
				debian_module.PropertyControl:      pck.Control,
			},
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion, packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusBadRequest, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := debian_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, distribution, component, pck.Architecture); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

func DownloadPackageFile(ctx *context.Context) {
	name := ctx.Params("name")
	version := ctx.Params("version")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeDebian,
			Name:        name,
			Version:     version,
		},
		&packages_service.PackageFileInfo{
			Filename:     fmt.Sprintf("%s_%s_%s.deb", name, version, ctx.Params("architecture")),
			CompositeKey: fmt.Sprintf("%s|%s", ctx.Params("distribution"), ctx.Params("component")),
		},
	)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		ContentType:  "application/vnd.debian.binary-package",
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

func DeletePackageFile(ctx *context.Context) {
	distribution := ctx.Params("distribution")
	component := ctx.Params("component")
	name := ctx.Params("name")
	version := ctx.Params("version")
	architecture := ctx.Params("architecture")

	owner := ctx.Package.Owner

	var pd *packages_model.PackageDescriptor

	err := db.WithTx(ctx, func(ctx stdctx.Context) error {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, owner.ID, packages_model.TypeDebian, name, version)
		if err != nil {
			return err
		}

		pf, err := packages_model.GetFileForVersionByName(
			ctx,
			pv.ID,
			fmt.Sprintf("%s_%s_%s.deb", name, version, architecture),
			fmt.Sprintf("%s|%s", distribution, component),
		)
		if err != nil {
			return err
		}

		if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
			return err
		}

		has, err := packages_model.HasVersionFileReferences(ctx, pv.ID)
		if err != nil {
			return err
		}
		if !has {
			pd, err = packages_model.GetPackageDescriptor(ctx, pv)
			if err != nil {
				return err
			}

			if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if pd != nil {
		notification.NotifyPackageDelete(ctx, ctx.Doer, pd)
	}

	if err := debian_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, distribution, component, architecture); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
