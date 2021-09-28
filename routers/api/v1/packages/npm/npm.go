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
	"code.gitea.io/gitea/modules/log"
	npm_module "code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"

	package_service "code.gitea.io/gitea/services/packages"
)

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.APIContext) {
	packageName, err := url.QueryUnescape(ctx.Params("id"))
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	packages, err := packages.GetPackagesByName(ctx.Repo.Repository.ID, packages.TypeNpm, packageName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	if len(packages) == 0 {
		ctx.Error(http.StatusNotFound, "", err)
		return
	}

	npmPackages, err := intializePackages(packages)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	resp := createPackageMetadataResponse(
		setting.AppURL+"api/v1/repos/"+ctx.Repo.Repository.FullName()+"/packages/npm",
		npmPackages,
	)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	packageName, err := url.QueryUnescape(ctx.Params("id"))
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := package_service.GetFileStreamByPackageNameAndVersion(ctx.Repo.Repository, packages.TypeNpm, packageName, packageVersion, filename)
	if err != nil {
		if err == packages.ErrPackageNotExist || err == packages.ErrPackageFileNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackage creates a new package
func UploadPackage(ctx *context.APIContext) {
	npmPackage, err := npm_module.ParsePackage(ctx.Req.Body)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		packages.TypeNpm,
		npmPackage.Name,
		npmPackage.Version,
		npmPackage.Metadata,
		false,
	)
	if err != nil {
		if err == packages.ErrDuplicatePackage {
			ctx.Error(http.StatusBadRequest, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	_, err = package_service.AddFileToPackage(p, npmPackage.Filename, int64(len(npmPackage.Data)), bytes.NewReader(npmPackage.Data))
	if err != nil {
		if err := packages.DeletePackageByID(p.ID); err != nil {
			log.Error("Error deleting package by id: %v", err)
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}
