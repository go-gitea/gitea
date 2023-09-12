// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"fmt"
	"net/http"

	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	debian_service "code.gitea.io/gitea/services/packages/debian"
	generic_service "code.gitea.io/gitea/services/packages/generic"
	rpm_service "code.gitea.io/gitea/services/packages/rpm"
)

var UploadTypeList = []packages_model.Type{
	packages_model.TypeDebian,
	packages_model.TypeGeneric,
	packages_model.TypeRpm,
}

func servePackageUploadError(ctx *context.Context, err error, packageType, repo string) {
	ctx.Flash.Error(err.Error())

	if repo == "" {
		ctx.Redirect(fmt.Sprintf("%s/-/packages/upload/%s", ctx.ContextUser.HTMLURL(), packageType))
	} else {
		ctx.Redirect(fmt.Sprintf("%s/-/packages/upload/%s?repo=%s", ctx.ContextUser.HTMLURL(), packageType, repo))
	}
}

func addRepoToUploadedPackage(ctx *context.Context, packageType, repoName string, packageID int64) bool {
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ctx.ContextUser.Name, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			servePackageUploadError(ctx, fmt.Errorf("repo not found"), packageType, repoName)
			return false
		}

		ctx.ServerError("GetRepositoryByOwnerAndName", err)
		return false
	}

	err = packages_model.SetRepositoryLink(ctx, packageID, repo.ID)
	if err != nil {
		ctx.ServerError("SetRepositoryLink", err)
		return false
	}

	return true
}

func UploadGenericPackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadGenericForm)
	upload, err := form.PackageFile.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}

	var filename string
	if form.PackageFilename == "" {
		filename = form.PackageFile.Filename
	} else {
		filename = form.PackageFilename
	}

	statusCode, pv, err := generic_service.UploadGenericPackage(ctx, upload, form.PackageName, form.PackageVersion, filename)
	if err != nil {
		if statusCode == http.StatusInternalServerError {
			ctx.ServerError("UploadGenericPackage", err)
			return
		}

		servePackageUploadError(ctx, err, "generic", form.PackageRepo)
		return
	}

	if form.PackageRepo != "" {
		if !addRepoToUploadedPackage(ctx, "generic", form.PackageRepo, pv.PackageID) {
			return
		}
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		ctx.ServerError("GetPackageDescriptor", err)
		return
	}

	ctx.Redirect(pd.FullWebLink())
}

func UploadDebianPackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadDebianForm)
	upload, err := form.PackageFile.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}

	statusCode, pv, err := debian_service.UploadDebianPackage(ctx, upload, form.PackageDistribution, form.PackageComponent)
	if err != nil {
		if statusCode == http.StatusInternalServerError {
			ctx.ServerError("UploadGenericPackage", err)
			return
		}

		servePackageUploadError(ctx, err, "debian", form.PackageRepo)
		return
	}

	if form.PackageRepo != "" {
		if !addRepoToUploadedPackage(ctx, "debian", form.PackageRepo, pv.PackageID) {
			return
		}
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		ctx.ServerError("GetPackageDescriptor", err)
		return
	}

	ctx.Redirect(pd.FullWebLink())
}

func UploadRpmPackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadRpmForm)
	upload, err := form.PackageFile.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}

	statusCode, pv, err := rpm_service.UploadRpmPackage(ctx, upload)
	if err != nil {
		if statusCode == http.StatusInternalServerError {
			ctx.ServerError("UploadRpmPackage", err)
			return
		}

		servePackageUploadError(ctx, err, "rpm", form.PackageRepo)
		return
	}

	if form.PackageRepo != "" {
		if !addRepoToUploadedPackage(ctx, "rpm", form.PackageRepo, pv.PackageID) {
			return
		}
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		ctx.ServerError("GetPackageDescriptor", err)
		return
	}

	ctx.Redirect(pd.FullWebLink())
}
