// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package composer

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	packages_module "code.gitea.io/gitea/modules/packages"
	composer_module "code.gitea.io/gitea/modules/packages/composer"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		type Error struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		}
		ctx.JSON(status, struct {
			Errors []Error `json:"errors"`
		}{
			Errors: []Error{
				{Status: status, Message: message},
			},
		})
	})
}

// ServiceIndex displays registry endpoints
func ServiceIndex(ctx *context.Context) {
	resp := createServiceIndexResponse(setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/composer")

	ctx.JSON(http.StatusOK, resp)
}

// SearchPackages searches packages, only "q" is supported
// https://packagist.org/apidoc#search-packages
func SearchPackages(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page < 1 {
		page = 1
	}
	perPage := ctx.FormInt("per_page")
	paginator := db.ListOptions{
		Page:     page,
		PageSize: convert.ToCorrectPageSize(perPage),
	}

	opts := &packages_model.PackageSearchOptions{
		OwnerID:   ctx.Package.Owner.ID,
		Type:      string(packages_model.TypeComposer),
		Query:     ctx.FormTrim("q"),
		Paginator: &paginator,
	}
	if ctx.FormTrim("type") != "" {
		opts.Properties = map[string]string{
			composer_module.TypeProperty: ctx.FormTrim("type"),
		}
	}

	pvs, total, err := packages_model.SearchLatestVersions(ctx, opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	nextLink := ""
	if len(pvs) == paginator.PageSize {
		u, err := url.Parse(setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/composer/search.json")
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		q := u.Query()
		q.Set("q", ctx.FormTrim("q"))
		q.Set("type", ctx.FormTrim("type"))
		q.Set("page", strconv.Itoa(page+1))
		if perPage != 0 {
			q.Set("per_page", strconv.Itoa(perPage))
		}
		u.RawQuery = q.Encode()

		nextLink = u.String()
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createSearchResultResponse(total, pds, nextLink)

	ctx.JSON(http.StatusOK, resp)
}

// EnumeratePackages lists all package names
// https://packagist.org/apidoc#list-packages
func EnumeratePackages(ctx *context.Context) {
	ps, err := packages_model.GetPackagesByType(db.DefaultContext, ctx.Package.Owner.ID, packages_model.TypeComposer)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	names := make([]string, 0, len(ps))
	for _, p := range ps {
		names = append(names, p.Name)
	}

	ctx.JSON(http.StatusOK, map[string][]string{
		"packageNames": names,
	})
}

// PackageMetadata returns the metadata for a single package
// https://packagist.org/apidoc#get-package-data
func PackageMetadata(ctx *context.Context) {
	vendorName := ctx.Params("vendorname")
	projectName := ctx.Params("projectname")

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeComposer, vendorName+"/"+projectName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, packages_model.ErrPackageNotExist)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createPackageMetadataResponse(
		setting.AppURL+"api/packages/"+ctx.Package.Owner.Name+"/composer",
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.Context) {
	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeComposer,
			Name:        ctx.Params("package"),
			Version:     ctx.Params("version"),
		},
		&packages_service.PackageFileInfo{
			Filename: ctx.Params("filename"),
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
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
func UploadPackage(ctx *context.Context) {
	buf, err := packages_module.CreateHashedBufferFromReader(ctx.Req.Body, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	cp, err := composer_module.ParsePackage(buf, buf.Size())
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if cp.Version == "" {
		v, err := version.NewVersion(ctx.FormTrim("version"))
		if err != nil {
			apiError(ctx, http.StatusBadRequest, composer_module.ErrInvalidVersion)
			return
		}
		cp.Version = v.String()
	}

	_, _, err = packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeComposer,
				Name:        cp.Name,
				Version:     cp.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.User,
			Metadata:         cp.Metadata,
			Properties: map[string]string{
				composer_module.TypeProperty: cp.Type,
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s.%s.zip", strings.ReplaceAll(cp.Name, "/", "-"), cp.Version)),
			},
			Data:   buf,
			IsLead: true,
		},
	)
	if err != nil {
		if err == packages_model.ErrDuplicatePackageVersion {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusCreated)
}
