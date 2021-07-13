// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	//"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util/filebuffer"

	package_service "code.gitea.io/gitea/services/packages"
)

// ServiceIndex https://docs.microsoft.com/en-us/nuget/api/service-index
func ServiceIndex(ctx *context.APIContext) {
	repoName := ctx.Repo.Repository.FullName()

	type resource struct {
		ID   string `json:"@id"`
		Type string `json:"@type"`
	}

	serviceIndex := struct {
		Version   string     `json:"version"`
		Resources []resource `json:"resources"`
	}{
		Version: "3.0.0",
		Resources: []resource{
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/query", Type: "SearchQueryService"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/query", Type: "SearchQueryService/3.0.0-beta"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/query", Type: "SearchQueryService/3.0.0-rc"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/registration", Type: "RegistrationsBaseUrl"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/registration", Type: "RegistrationsBaseUrl/3.0.0-beta"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/registration", Type: "RegistrationsBaseUrl/3.0.0-rc"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget/package", Type: "PackageBaseAddress/3.0.0"},
			{ID: setting.AppURL + "api/v1/repos/" + repoName + "/packages/nuget", Type: "PackagePublish/2.0.0"},
		},
	}

	ctx.JSON(http.StatusOK, serviceIndex)
}

// SearchService https://docs.microsoft.com/en-us/nuget/api/search-query-service-resource#search-for-packages
func SearchService(ctx *context.APIContext) {
	query := ctx.QueryTrim("q")
	skip := ctx.QueryInt("skip")
	take := ctx.QueryInt("take")

	total, packages, err := models.SearchPackages(ctx.Repo.Repository.ID, models.PackageNuGet, query, skip, take)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	nugetPackages, err := intializePackages(packages)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	resp := createSearchResultResponse(
		&linkBuilder{setting.AppURL + "api/v1/repos/" + ctx.Repo.Repository.FullName() + "/packages/nuget"},
		total,
		nugetPackages,
	)

	ctx.JSON(http.StatusOK, resp)
}

// RegistrationIndex https://docs.microsoft.com/en-us/nuget/api/registration-base-url-resource#registration-index
func RegistrationIndex(ctx *context.APIContext) {
	packageName := ctx.Params("id")

	packages, err := models.GetPackagesByName(ctx.Repo.Repository.ID, models.PackageNuGet, packageName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	if len(packages) == 0 {
		ctx.Error(http.StatusNotFound, "", err)
		return
	}

	nugetPackages, err := intializePackages(packages)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
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

	p, err := models.GetPackageByNameAndVersion(ctx.Repo.Repository.ID, models.PackageNuGet, packageName, packageVersion)
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	nugetPackage, err := intializePackage(p)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
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

	packages, err := models.GetPackagesByName(ctx.Repo.Repository.ID, models.PackageNuGet, packageName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	if len(packages) == 0 {
		ctx.Error(http.StatusNotFound, "", err)
		return
	}

	nugetPackages, err := intializePackages(packages)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	resp := createPackageVersionsResponse(nugetPackages)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageContent https://docs.microsoft.com/en-us/nuget/api/package-base-address-resource#download-package-content-nupkg
func DownloadPackageContent(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := package_service.GetPackageFileStream(ctx.Repo.Repository, models.PackageGeneric, packageName, packageVersion, filename)
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackage creates a new package with the metadata contained in the uploaded nupgk file
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#push-a-package
func UploadPackage(ctx *context.APIContext) {
	upload, err := ctx.UploadStream()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}
	defer upload.Close()

	buf, err := filebuffer.CreateFromReader(upload, 32*1024*1024)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	defer buf.Close()

	meta, err := nuget_module.ParsePackageMetaData(buf, buf.Size())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		models.PackageNuGet,
		meta.ID,
		meta.Version,
		meta,
		false,
	)
	if err != nil {
		if err == models.ErrDuplicatePackage {
			ctx.Error(http.StatusBadRequest, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	filename := strings.ToLower(fmt.Sprintf("%s.%s.nupkg", meta.ID, meta.Version))
	_, err = package_service.AddFileToPackage(p, filename, buf.Size(), buf)
	if err != nil {
		if err := models.DeletePackageByID(p.ID); err != nil {
			log.Error("Error deleting package by id: %v", err)
		}
		ctx.Error(http.StatusInternalServerError, "", err)
	}

	ctx.PlainText(http.StatusCreated, nil)
}

// DeletePackage heard deletes the package
// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#delete-a-package
func DeletePackage(ctx *context.APIContext) {
	packageName := ctx.Params("id")
	packageVersion := ctx.Params("version")

	err := package_service.DeletePackage(ctx.Repo.Repository, models.PackageGeneric, packageName, packageVersion)
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", "")
	}
}
