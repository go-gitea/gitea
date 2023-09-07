// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"errors"
	"fmt"
	"io"

	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	packages_service "code.gitea.io/gitea/services/packages"
	debian_service "code.gitea.io/gitea/services/packages/debian"
)

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

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		ctx.ServerError("CreateHashedBufferFromReader", err)
		return
	}
	defer buf.Close()

	var filename string
	if form.PackageFilename == "" {
		filename = form.PackageFile.Filename
	} else {
		filename = form.PackageFilename
	}

	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeGeneric,
				Name:        form.PackageName,
				Version:     form.PackageVersion,
			},
			Creator: ctx.Doer,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			servePackageUploadError(ctx, err, "generic", form.PackageRepo)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			servePackageUploadError(ctx, err, "generic", form.PackageRepo)
		default:
			ctx.ServerError("CreatePackageOrAddFileToExisting", err)
		}
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

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		ctx.ServerError("GetGenericPackageFile", err)
		return
	}
	defer buf.Close()

	pck, err := debian_module.ParsePackage(buf)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			servePackageUploadError(ctx, err, "debian", form.PackageRepo)
		} else {
			ctx.ServerError("ParsePackage", err)
		}
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		ctx.ServerError("SeekBuffer", err)
		return
	}

	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
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
				CompositeKey: fmt.Sprintf("%s|%s", form.PackageDistribution, form.PackageComponent),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				debian_module.PropertyDistribution: form.PackageDistribution,
				debian_module.PropertyComponent:    form.PackageComponent,
				debian_module.PropertyArchitecture: pck.Architecture,
				debian_module.PropertyControl:      pck.Control,
			},
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion, packages_model.ErrDuplicatePackageFile:
			servePackageUploadError(ctx, err, "debian", form.PackageRepo)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			servePackageUploadError(ctx, err, "debian", form.PackageRepo)
		default:
			ctx.ServerError("CreatePackageOrAddFileToExisting", err)
		}
		return
	}

	if err := debian_service.BuildSpecificRepositoryFiles(ctx, ctx.Package.Owner.ID, form.PackageDistribution, form.PackageComponent, pck.Architecture); err != nil {
		ctx.ServerError("BuildSpecificRepositoryFiles", err)
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
