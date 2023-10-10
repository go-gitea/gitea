// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"fmt"

	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	alpine_service "code.gitea.io/gitea/services/packages/alpine"
	debian_service "code.gitea.io/gitea/services/packages/debian"
	generic_service "code.gitea.io/gitea/services/packages/generic"
	rpm_service "code.gitea.io/gitea/services/packages/rpm"
)

var UploadTypeList = []packages_model.Type{
	packages_model.TypeAlpine,
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

func uploadPackageFinish(ctx *context.Context, packageType, packageRepo string, pv *packages_model.PackageVersion) {
	if packageRepo != "" {
		if !addRepoToUploadedPackage(ctx, packageType, packageRepo, pv.PackageID) {
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

func UploadAlpinePackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadAlpineForm)
	upload, err := form.File.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}
	defer upload.Close()

	userError, pv, err := alpine_service.UploadAlpinePackage(ctx, upload, form.Repo, form.Branch)
	if err != nil {
		if !userError {
			ctx.ServerError("UploadAlpinePackage", err)
			return
		}

		servePackageUploadError(ctx, err, "alpine", form.Repo)
		return
	}

	uploadPackageFinish(ctx, "alpine", form.Repo, pv)
}

func UploadDebianPackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadDebianForm)
	upload, err := form.File.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}
	defer upload.Close()

	userError, pv, err := debian_service.UploadDebianPackage(ctx, upload, form.Distribution, form.Component)
	if err != nil {
		if !userError {
			ctx.ServerError("UploadDebianPackage", err)
			return
		}

		servePackageUploadError(ctx, err, "debian", form.Repo)
		return
	}

	uploadPackageFinish(ctx, "debian", form.Repo, pv)
}

func UploadGenericPackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadGenericForm)
	upload, err := form.File.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}
	defer upload.Close()

	var filename string
	if form.Filename == "" {
		filename = form.File.Filename
	} else {
		filename = form.Filename
	}

	userError, pv, err := generic_service.UploadGenericPackage(ctx, upload, form.Name, form.Version, filename)
	if err != nil {
		if !userError {
			ctx.ServerError("UploadGenericPackage", err)
			return
		}

		servePackageUploadError(ctx, err, "generic", form.Repo)
		return
	}

	uploadPackageFinish(ctx, "debian", form.Repo, pv)
}

func UploadRpmPackagePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.PackageUploadRpmForm)
	upload, err := form.File.Open()
	if err != nil {
		ctx.ServerError("GetPackageFile", err)
		return
	}
	defer upload.Close()

	userError, pv, err := rpm_service.UploadRpmPackage(ctx, upload)
	if err != nil {
		if !userError {
			ctx.ServerError("UploadRpmPackage", err)
			return
		}

		servePackageUploadError(ctx, err, "rpm", form.Repo)
		return
	}

	uploadPackageFinish(ctx, "rpm", form.Repo, pv)
}
