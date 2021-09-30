// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util/filebuffer"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	package_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, map[string]string{
			"Message": message,
		})
	})
}

// ServiceIndex https://docs.microsoft.com/en-us/nuget/api/service-index
func ServiceIndex(ctx *context.APIContext) {
	repoName := ctx.Repo.Repository.FullName()

	resp := createServiceIndexResponse(setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget")

	ctx.JSON(http.StatusOK, resp)
}

// SearchService https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-for-packages
func SearchService(ctx *context.APIContext) {
	packages, count, err := packages.GetPackages(&packages.PackageSearchOptions{
		RepoID: ctx.Repo.Repository.ID,
		Type:   "nuget",
		Query:  ctx.FormTrim("q"),
		Paginator: db.NewAbsoluteListOptions(
			ctx.FormInt("skip"),
			ctx.FormInt("take"),
		),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	nugetPackages, err := intializePackages(packages)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createSearchResultResponse(
		&linkBuilder{setting.AppURL + "api/v1/repos/" + ctx.Repo.Repository.FullName() + "/packages/nuget"},
		count,
		nugetPackages,
	)

	ctx.JSON(http.StatusOK, resp)
}

// RegistrationIndex https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-index
func RegistrationIndex(ctx *context.APIContext) {
	packageName := ctx.Params("id")

	packages, err := packages.GetPackagesByName(ctx.Repo.Repository.ID, packages.TypeNuGet, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(packages) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	nugetPackages, err := intializePackages(packages)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createRegistrationIndexResponse(
		&linkBuilder{setting.AppURL + "api/v1/repos/" + ctx.Repo.Repository.FullName() + "/packages/nuget"},
		nugetPackages,
	)

	ctx.JSON(http.StatusOK, resp)
}

// RegistrationLeaf https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-leaf
func RegistrationLeaf(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := strings.TrimSuffix(ctx.Params("version"), ".json")

	p, err := packages.GetPackageByNameAndVersion(ctx.Repo.Repository.ID, packages.TypeNuGet, packageName, packageVersion)
	if err != nil {
		if err == packages.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	nugetPackage, err := intializePackage(p)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createRegistrationLeafResponse(
		&linkBuilder{setting.AppURL + "api/v1/repos/" + ctx.Repo.Repository.FullName() + "/packages/nuget"},
		nugetPackage,
	)

	ctx.JSON(http.StatusOK, resp)
}

// EnumeratePackageVersions https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#enumerate-package-versions
func EnumeratePackageVersions(ctx *context.APIContext) {
	packageName := ctx.Params("id")

	packages, err := packages.GetPackagesByName(ctx.Repo.Repository.ID, packages.TypeNuGet, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(packages) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	nugetPackages, err := intializePackages(packages)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createPackageVersionsResponse(nugetPackages)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#download-package-content-nupkg
func DownloadPackageFile(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := package_service.GetFileStreamByPackageNameAndVersion(ctx.Repo.Repository, packages.TypeNuGet, packageName, packageVersion, filename)
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

// UploadPackage creates a new package with the metadata contained in the uploaded nupgk file
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#push-a-package
func UploadPackage(ctx *context.APIContext) {
	meta, buf, closables := processUploadedFile(ctx, nuget_module.DependencyPackage)
	defer func() {
		for _, c := range closables {
			c.Close()
		}
	}()
	if meta == nil {
		return
	}

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		packages.TypeNuGet,
		meta.ID,
		meta.Version,
		meta,
		false,
	)
	if err != nil {
		if err == packages.ErrDuplicatePackage {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	filename := strings.ToLower(fmt.Sprintf("%s.%s.nupkg", meta.ID, meta.Version))
	_, err = package_service.AddFileToPackage(p, filename, buf.Size(), buf)
	if err != nil {
		if err := packages.DeletePackageByID(p.ID); err != nil {
			log.Error("Error deleting package by id: %v", err)
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}

// UploadSymbolPackage adds a symbol package to an existing package
// https://docs.microsoft.com/en-us/nuget/api/symbol-package-publish-resource
func UploadSymbolPackage(ctx *context.APIContext) {
	meta, buf, closables := processUploadedFile(ctx, nuget_module.SymbolsPackage)
	defer func() {
		for _, c := range closables {
			c.Close()
		}
	}()
	if meta == nil {
		return
	}

	p, err := packages.GetPackageByNameAndVersion(ctx.Repo.Repository.ID, packages.TypeNuGet, meta.ID, meta.Version)
	if err != nil {
		if err == packages.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	filename := strings.ToLower(fmt.Sprintf("%s.%s.snupkg", meta.ID, meta.Version))
	_, err = package_service.AddFileToPackage(p, filename, buf.Size(), buf)
	if err != nil {
		if err == packages.ErrDuplicatePackageFile {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}

func processUploadedFile(ctx *context.APIContext, expectedType nuget_module.PackageType) (*nuget_module.Metadata, *filebuffer.FileBackedBuffer, []io.Closer) {
	closables := make([]io.Closer, 0, 2)

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return nil, nil, closables
	}

	if close {
		closables = append(closables, upload)
	}

	buf, err := filebuffer.CreateFromReader(upload, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return nil, nil, closables
	}
	closables = append(closables, buf)

	meta, err := nuget_module.ParsePackageMetaData(buf, buf.Size())
	if err != nil {
		if err == nuget_module.ErrMissingNuspecFile || err == nuget_module.ErrNuspecFileTooLarge || err == nuget_module.ErrNuspecInvalidID || err == nuget_module.ErrNuspecInvalidVersion {
			apiError(ctx, http.StatusBadRequest, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return nil, nil, closables
	}
	if meta.PackageType != expectedType {
		apiError(ctx, http.StatusBadRequest, errors.New("unexpected package type"))
		return nil, nil, closables
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return nil, nil, closables
	}
	return meta, buf, closables
}

// DeletePackage hard deletes the package
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#delete-a-package
func DeletePackage(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")

	err := package_service.DeletePackageByNameAndVersion(ctx.User, ctx.Repo.Repository, packages.TypeNuGet, packageName, packageVersion)
	if err != nil {
		if err == packages.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
	}
}
