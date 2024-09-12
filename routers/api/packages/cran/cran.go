// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	cran_model "code.gitea.io/gitea/models/packages/cran"
	packages_module "code.gitea.io/gitea/modules/packages"
	cran_module "code.gitea.io/gitea/modules/packages/cran"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func EnumerateSourcePackages(ctx *context.Context) {
	enumeratePackages(ctx, ctx.PathParam("format"), &cran_model.SearchOptions{
		OwnerID:  ctx.Package.Owner.ID,
		FileType: cran_module.TypeSource,
	})
}

func EnumerateBinaryPackages(ctx *context.Context) {
	enumeratePackages(ctx, ctx.PathParam("format"), &cran_model.SearchOptions{
		OwnerID:  ctx.Package.Owner.ID,
		FileType: cran_module.TypeBinary,
		Platform: ctx.PathParam("platform"),
		RVersion: ctx.PathParam("rversion"),
	})
}

func enumeratePackages(ctx *context.Context, format string, opts *cran_model.SearchOptions) {
	if format != "" && format != ".gz" {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pvs, err := cran_model.SearchLatestVersions(ctx, opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	var w io.Writer = ctx.Resp

	if format == ".gz" {
		ctx.Resp.Header().Set("Content-Type", "application/x-gzip")

		gzw := gzip.NewWriter(w)
		defer gzw.Close()

		w = gzw
	} else {
		ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	}
	ctx.Resp.WriteHeader(http.StatusOK)

	for i, pd := range pds {
		if i > 0 {
			fmt.Fprintln(w)
		}

		var pfd *packages_model.PackageFileDescriptor
		for _, d := range pd.Files {
			if d.Properties.GetByName(cran_module.PropertyType) == opts.FileType &&
				d.Properties.GetByName(cran_module.PropertyPlatform) == opts.Platform &&
				d.Properties.GetByName(cran_module.PropertyRVersion) == opts.RVersion {
				pfd = d
				break
			}
		}

		metadata := pd.Metadata.(*cran_module.Metadata)

		fmt.Fprintln(w, "Package:", pd.Package.Name)
		fmt.Fprintln(w, "Version:", pd.Version.Version)
		if metadata.License != "" {
			fmt.Fprintln(w, "License:", metadata.License)
		}
		if len(metadata.Depends) > 0 {
			fmt.Fprintln(w, "Depends:", strings.Join(metadata.Depends, ", "))
		}
		if len(metadata.Imports) > 0 {
			fmt.Fprintln(w, "Imports:", strings.Join(metadata.Imports, ", "))
		}
		if len(metadata.LinkingTo) > 0 {
			fmt.Fprintln(w, "LinkingTo:", strings.Join(metadata.LinkingTo, ", "))
		}
		if len(metadata.Suggests) > 0 {
			fmt.Fprintln(w, "Suggests:", strings.Join(metadata.Suggests, ", "))
		}
		needsCompilation := "no"
		if metadata.NeedsCompilation {
			needsCompilation = "yes"
		}
		fmt.Fprintln(w, "NeedsCompilation:", needsCompilation)
		fmt.Fprintln(w, "MD5sum:", pfd.Blob.HashMD5)
	}
}

func UploadSourcePackageFile(ctx *context.Context) {
	uploadPackageFile(
		ctx,
		packages_model.EmptyFileKey,
		map[string]string{
			cran_module.PropertyType: cran_module.TypeSource,
		},
	)
}

func UploadBinaryPackageFile(ctx *context.Context) {
	platform, rversion := ctx.FormTrim("platform"), ctx.FormTrim("rversion")
	if platform == "" || rversion == "" {
		apiError(ctx, http.StatusBadRequest, nil)
		return
	}

	uploadPackageFile(
		ctx,
		platform+"|"+rversion,
		map[string]string{
			cran_module.PropertyType:     cran_module.TypeBinary,
			cran_module.PropertyPlatform: platform,
			cran_module.PropertyRVersion: rversion,
		},
	)
}

func uploadPackageFile(ctx *context.Context, compositeKey string, properties map[string]string) {
	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
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

	pck, err := cran_module.ParsePackage(buf, buf.Size())
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
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeCran,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			SemverCompatible: false,
			Creator:          ctx.Doer,
			Metadata:         pck.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     fmt.Sprintf("%s_%s%s", pck.Name, pck.Version, pck.FileExtension),
				CompositeKey: compositeKey,
			},
			Creator:    ctx.Doer,
			Data:       buf,
			IsLead:     true,
			Properties: properties,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

func DownloadSourcePackageFile(ctx *context.Context) {
	downloadPackageFile(ctx, &cran_model.SearchOptions{
		OwnerID:  ctx.Package.Owner.ID,
		FileType: cran_module.TypeSource,
		Filename: ctx.PathParam("filename"),
	})
}

func DownloadBinaryPackageFile(ctx *context.Context) {
	downloadPackageFile(ctx, &cran_model.SearchOptions{
		OwnerID:  ctx.Package.Owner.ID,
		FileType: cran_module.TypeBinary,
		Platform: ctx.PathParam("platform"),
		RVersion: ctx.PathParam("rversion"),
		Filename: ctx.PathParam("filename"),
	})
}

func downloadPackageFile(ctx *context.Context, opts *cran_model.SearchOptions) {
	pf, err := cran_model.SearchFile(ctx, opts)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pf)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}
