// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package npm

import (
	"bytes"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	package_module "code.gitea.io/gitea/modules/packages"
	npm_module "code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	packages_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, map[string]string{
			"error": message,
		})
	})
}

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.APIContext) {
	packageName, err := url.QueryUnescape(ctx.Params("id"))
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	pvs, err := packages.GetVersionsByPackageName(ctx.Repo.Repository.ID, packages.TypeNpm, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createPackageMetadataResponse(
		setting.AppURL+"api/v1/repos/"+ctx.Repo.Repository.FullName()+"/packages/npm",
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	packageName, err := url.QueryUnescape(ctx.Params("id"))
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(ctx.Repo.Repository, packages.TypeNpm, packageName, packageVersion, filename)
	if err != nil {
		if err == packages.ErrPackageNotExist || err == packages.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackage creates a new package
func UploadPackage(ctx *context.APIContext) {
	npmPackage, err := npm_module.ParsePackage(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	buf, err := package_module.CreateHashedBufferFromReader(bytes.NewReader(npmPackage.Data), 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	_, _, err = packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Repository:  ctx.Repo.Repository,
				PackageType: packages.TypeNpm,
				Name:        npmPackage.Name,
				Version:     npmPackage.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.User,
			Metadata:         npmPackage.Metadata,
		},
		&packages_service.PackageFileInfo{
			Filename: npmPackage.Filename,
			Data:     buf,
			IsLead:   true,
		},
	)
	if err != nil {
		if err == packages.ErrDuplicatePackageVersion {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}
