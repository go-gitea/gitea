// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	stdctx "context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/routers/api/packages/helper"
	notify_service "code.gitea.io/gitea/services/notify"
	packages_service "code.gitea.io/gitea/services/packages"
	debian_service "code.gitea.io/gitea/services/packages/debian"
)

func apiError(ctx *context.Context, obj any, statuses ...int) {
	status := helper.FormResponseCode(obj, statuses...)
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := debian_service.GetOrCreateKeyPair(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, err)
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
	pv, err := debian_service.GetOrCreateRepositoryVersion(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, err)
		return
	}

	key := ctx.Params("distribution")

	component := ctx.Params("component")
	architecture := strings.TrimPrefix(ctx.Params("architecture"), "binary-")
	if component != "" && architecture != "" {
		key += "|" + component + "|" + architecture
	}

	s, u, pf, err := packages_service.GetFileStreamByPackageVersion(
		ctx,
		pv,
		&packages_service.PackageFileInfo{
			Filename:     ctx.Params("filename"),
			CompositeKey: key,
		},
	)
	if err != nil {
		apiError(ctx, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

// https://wiki.debian.org/DebianRepository/Format#indices_acquisition_via_hashsums_.28by-hash.29
func GetRepositoryFileByHash(ctx *context.Context) {
	pv, err := debian_service.GetOrCreateRepositoryVersion(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, err)
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
		apiError(ctx, err)
		return
	}
	if len(pfs) != 1 {
		apiError(ctx, nil, http.StatusNotFound)
		return
	}

	s, u, pf, err := packages_service.GetPackageFileStream(ctx, pfs[0])
	if err != nil {
		apiError(ctx, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

func UploadPackageFile(ctx *context.Context) {
	distribution := strings.TrimSpace(ctx.Params("distribution"))
	component := strings.TrimSpace(ctx.Params("component"))
	if distribution == "" || component == "" {
		apiError(ctx, "invalid distribution or component", http.StatusBadRequest)
		return
	}

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, err)
		return
	}
	defer buf.Close()

	pck, err := debian_module.ParsePackage(buf)
	if err != nil {
		apiError(ctx, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, err)
		return
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
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
		apiError(ctx, err)
		return
	}

	if err := debian_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, distribution, component, pck.Architecture); err != nil {
		apiError(ctx, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

func DownloadPackageFile(ctx *context.Context) {
	name := ctx.Params("name")
	version := ctx.Params("version")

	s, u, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
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
		apiError(ctx, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf, &context.ServeHeaderOptions{
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
		apiError(ctx, err)
		return
	}

	if pd != nil {
		notify_service.PackageDelete(ctx, ctx.Doer, pd)
	}

	if err := debian_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, distribution, component, architecture); err != nil {
		apiError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
