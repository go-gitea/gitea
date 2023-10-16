// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chef

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	chef_module "code.gitea.io/gitea/modules/packages/chef"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.Context, status int, obj any) {
	type Error struct {
		ErrorMessages []string `json:"error_messages"`
	}

	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, Error{
			ErrorMessages: []string{message},
		})
	})
}

func PackagesUniverse(ctx *context.Context) {
	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeChef,
		IsInternal: util.OptionalBoolFalse,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type VersionInfo struct {
		LocationType string            `json:"location_type"`
		LocationPath string            `json:"location_path"`
		DownloadURL  string            `json:"download_url"`
		Dependencies map[string]string `json:"dependencies"`
	}

	baseURL := setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/chef/api/v1"

	universe := make(map[string]map[string]*VersionInfo)
	for _, pd := range pds {
		if _, ok := universe[pd.Package.Name]; !ok {
			universe[pd.Package.Name] = make(map[string]*VersionInfo)
		}
		universe[pd.Package.Name][pd.Version.Version] = &VersionInfo{
			LocationType: "opscode",
			LocationPath: baseURL,
			DownloadURL:  fmt.Sprintf("%s/cookbooks/%s/versions/%s/download", baseURL, url.PathEscape(pd.Package.Name), pd.Version.Version),
			Dependencies: pd.Metadata.(*chef_module.Metadata).Dependencies,
		}
	}

	ctx.JSON(http.StatusOK, universe)
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_list.rb
// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_search.rb
func EnumeratePackages(ctx *context.Context) {
	opts := &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeChef,
		Name:       packages_model.SearchValue{Value: ctx.FormTrim("q")},
		IsInternal: util.OptionalBoolFalse,
		Paginator: db.NewAbsoluteListOptions(
			ctx.FormInt("start"),
			ctx.FormInt("items"),
		),
	}

	switch strings.ToLower(ctx.FormTrim("order")) {
	case "recently_updated", "recently_added":
		opts.Sort = packages_model.SortCreatedDesc
	default:
		opts.Sort = packages_model.SortNameAsc
	}

	pvs, total, err := packages_model.SearchLatestVersions(ctx, opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type Item struct {
		CookbookName        string `json:"cookbook_name"`
		CookbookMaintainer  string `json:"cookbook_maintainer"`
		CookbookDescription string `json:"cookbook_description"`
		Cookbook            string `json:"cookbook"`
	}

	type Result struct {
		Start int     `json:"start"`
		Total int     `json:"total"`
		Items []*Item `json:"items"`
	}

	baseURL := setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/chef/api/v1/cookbooks/"

	items := make([]*Item, 0, len(pds))
	for _, pd := range pds {
		metadata := pd.Metadata.(*chef_module.Metadata)

		items = append(items, &Item{
			CookbookName:        pd.Package.Name,
			CookbookMaintainer:  metadata.Author,
			CookbookDescription: metadata.Description,
			Cookbook:            baseURL + url.PathEscape(pd.Package.Name),
		})
	}

	skip, _ := opts.Paginator.GetSkipTake()

	ctx.JSON(http.StatusOK, &Result{
		Start: skip,
		Total: int(total),
		Items: items,
	})
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_show.rb
func PackageMetadata(ctx *context.Context) {
	packageName := ctx.Params("name")

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeChef, packageName)
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

	sort.Slice(pds, func(i, j int) bool {
		return pds[i].SemVer.LessThan(pds[j].SemVer)
	})

	type Result struct {
		Name          string    `json:"name"`
		Maintainer    string    `json:"maintainer"`
		Description   string    `json:"description"`
		Category      string    `json:"category"`
		LatestVersion string    `json:"latest_version"`
		SourceURL     string    `json:"source_url"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
		Deprecated    bool      `json:"deprecated"`
		Versions      []string  `json:"versions"`
	}

	baseURL := fmt.Sprintf("%sapi/packages/%s/chef/api/v1/cookbooks/%s/versions/", setting.AppURL, ctx.Package.Owner.Name, url.PathEscape(packageName))

	versions := make([]string, 0, len(pds))
	for _, pd := range pds {
		versions = append(versions, baseURL+pd.Version.Version)
	}

	latest := pds[len(pds)-1]

	metadata := latest.Metadata.(*chef_module.Metadata)

	ctx.JSON(http.StatusOK, &Result{
		Name:          latest.Package.Name,
		Maintainer:    metadata.Author,
		Description:   metadata.Description,
		LatestVersion: baseURL + latest.Version.Version,
		SourceURL:     metadata.RepositoryURL,
		CreatedAt:     latest.Version.CreatedUnix.AsLocalTime(),
		UpdatedAt:     latest.Version.CreatedUnix.AsLocalTime(),
		Deprecated:    false,
		Versions:      versions,
	})
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_show.rb
func PackageVersionMetadata(ctx *context.Context) {
	packageName := ctx.Params("name")
	packageVersion := strings.ReplaceAll(ctx.Params("version"), "_", ".") // Chef calls this endpoint with "_" instead of "."?!

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeChef, packageName, packageVersion)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type Result struct {
		Version         string            `json:"version"`
		TarballFileSize int64             `json:"tarball_file_size"`
		PublishedAt     time.Time         `json:"published_at"`
		Cookbook        string            `json:"cookbook"`
		File            string            `json:"file"`
		License         string            `json:"license"`
		Dependencies    map[string]string `json:"dependencies"`
	}

	baseURL := fmt.Sprintf("%sapi/packages/%s/chef/api/v1/cookbooks/%s", setting.AppURL, ctx.Package.Owner.Name, url.PathEscape(pd.Package.Name))

	metadata := pd.Metadata.(*chef_module.Metadata)

	ctx.JSON(http.StatusOK, &Result{
		Version:         pd.Version.Version,
		TarballFileSize: pd.Files[0].Blob.Size,
		PublishedAt:     pd.Version.CreatedUnix.AsLocalTime(),
		Cookbook:        baseURL,
		File:            fmt.Sprintf("%s/versions/%s/download", baseURL, pd.Version.Version),
		License:         metadata.License,
		Dependencies:    metadata.Dependencies,
	})
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_share.rb
func UploadPackage(ctx *context.Context) {
	file, _, err := ctx.Req.FormFile("tarball")
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	buf, err := packages_module.CreateHashedBufferFromReader(file)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pck, err := chef_module.ParsePackage(buf)
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
				PackageType: packages_model.TypeChef,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:          ctx.Doer,
			SemverCompatible: true,
			Metadata:         pck.Metadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(pck.Version + ".tar.gz"),
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

	ctx.JSON(http.StatusCreated, make(map[any]any))
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_download.rb
func DownloadPackage(ctx *context.Context) {
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeChef, ctx.Params("name"), ctx.Params("version"))
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pf := pd.Files[0].File

	s, u, _, err := packages_service.GetPackageFileStream(ctx, pf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_unshare.rb
func DeletePackageVersion(ctx *context.Context) {
	packageName := ctx.Params("name")
	packageVersion := ctx.Params("version")

	err := packages_service.RemovePackageVersionByNameAndVersion(
		ctx,
		ctx.Doer,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeChef,
			Name:        packageName,
			Version:     packageVersion,
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusOK)
}

// https://github.com/chef/chef/blob/main/knife/lib/chef/knife/supermarket_unshare.rb
func DeletePackage(ctx *context.Context) {
	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeChef, ctx.Params("name"))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	for _, pv := range pvs {
		if err := packages_service.RemovePackageVersion(ctx, ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.Status(http.StatusOK)
}
