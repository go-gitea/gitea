// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/optional"
	packages_module "code.gitea.io/gitea/modules/packages"
	goproxy_module "code.gitea.io/gitea/modules/packages/goproxy"
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

func EnumeratePackageVersions(ctx *context.Context) {
	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeGo, ctx.PathParam("name"))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	sort.Slice(pvs, func(i, j int) bool {
		return pvs[i].CreatedUnix < pvs[j].CreatedUnix
	})

	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")

	for _, pv := range pvs {
		fmt.Fprintln(ctx.Resp, pv.Version)
	}
}

func PackageVersionMetadata(ctx *context.Context) {
	pv, err := resolvePackage(ctx, ctx.Package.Owner.ID, ctx.PathParam("name"), ctx.PathParam("version"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.JSON(http.StatusOK, struct {
		Version string    `json:"Version"`
		Time    time.Time `json:"Time"`
	}{
		Version: pv.Version,
		Time:    pv.CreatedUnix.AsLocalTime(),
	})
}

func PackageVersionGoModContent(ctx *context.Context) {
	pv, err := resolvePackage(ctx, ctx.Package.Owner.ID, ctx.PathParam("name"), ctx.PathParam("version"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeVersion, pv.ID, goproxy_module.PropertyGoMod)
	if err != nil || len(pps) != 1 {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusOK, pps[0].Value)
}

func DownloadPackageFile(ctx *context.Context) {
	pv, err := resolvePackage(ctx, ctx.Package.Owner.ID, ctx.PathParam("name"), ctx.PathParam("version"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pfs, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
	if err != nil || len(pfs) != 1 {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pfs[0])
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	helper.ServePackageFile(ctx, s, u, pfs[0])
}

func resolvePackage(ctx *context.Context, ownerID int64, name, version string) (*packages_model.PackageVersion, error) {
	var pv *packages_model.PackageVersion

	if version == "latest" {
		pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
			OwnerID: ownerID,
			Type:    packages_model.TypeGo,
			Name: packages_model.SearchValue{
				Value:      name,
				ExactMatch: true,
			},
			IsInternal: optional.Some(false),
			Sort:       packages_model.SortCreatedDesc,
		})
		if err != nil {
			return nil, err
		}

		if len(pvs) != 1 {
			return nil, packages_model.ErrPackageNotExist
		}

		pv = pvs[0]
	} else {
		var err error
		pv, err = packages_model.GetVersionByNameAndVersion(ctx, ownerID, packages_model.TypeGo, name, version)
		if err != nil {
			return nil, err
		}
	}

	return pv, nil
}

func UploadPackage(ctx *context.Context) {
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

	pck, err := goproxy_module.ParsePackage(buf, buf.Size())
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

	_, _, err = packages_service.CreatePackageAndAddFile(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeGo,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator: ctx.Doer,
			VersionProperties: map[string]string{
				goproxy_module.PropertyGoMod: pck.GoMod,
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: fmt.Sprintf("%v.zip", pck.Version),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion:
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
