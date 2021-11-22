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
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	package_module "code.gitea.io/gitea/modules/packages"
	composer_module "code.gitea.io/gitea/modules/packages/composer"
	"code.gitea.io/gitea/modules/setting"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
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
func ServiceIndex(ctx *context.APIContext) {
	resp := createServiceIndexResponse(setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/composer")

	ctx.JSON(http.StatusOK, resp)
}

// SearchPackages searches packages, only "q" is supported
// https://packagist.org/apidoc#search-packages
func SearchPackages(ctx *context.APIContext) {
	page := ctx.FormInt("page")
	if page < 1 {
		page = 1
	}
	perPage := ctx.FormInt("per_page")
	paginator := db.ListOptions{
		Page:     page,
		PageSize: convert.ToCorrectPageSize(perPage),
	}

	opts := &packages.PackageSearchOptions{
		OwnerID:   ctx.Package.Owner.ID,
		Type:      string(packages.TypeComposer),
		Query:     ctx.FormTrim("q"),
		Paginator: &paginator,
	}
	if ctx.FormTrim("type") != "" {
		opts.Properties = map[string]string{
			composer_module.TypeProperty: ctx.FormTrim("type"),
		}
	}

	pvs, total, err := packages.SearchLatestVersions(opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	nextLink := ""
	if len(pvs) == paginator.PageSize {
		u, err := url.Parse(setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/composer/search.json")
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

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createSearchResultResponse(total, pds, nextLink)

	ctx.JSON(http.StatusOK, resp)
}

// EnumeratePackages lists all package names
// https://packagist.org/apidoc#list-packages
func EnumeratePackages(ctx *context.APIContext) {
	ps, err := packages.GetPackagesByType(db.DefaultContext, ctx.Package.Owner.ID, packages.TypeComposer)
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
func PackageMetadata(ctx *context.APIContext) {
	vendorName := ctx.Params("vendorname")
	projectName := ctx.Params("projectname")

	pvs, err := packages.GetVersionsByPackageName(ctx.Package.Owner.ID, packages.TypeComposer, vendorName+"/"+projectName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, packages.ErrPackageNotExist)
		return
	}

	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := createPackageMetadataResponse(
		setting.AppURL+"api/v1/packages/"+ctx.Package.Owner.Name+"/composer",
		pds,
	)

	ctx.JSON(http.StatusOK, resp)
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	versionID := ctx.ParamsInt64("versionid")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageVersionID(
		ctx.Package.Owner,
		versionID,
		filename,
	)
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
	buf, err := package_module.CreateHashedBufferFromReader(ctx.Req.Body, 32*1024*1024)
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
				PackageType: packages.TypeComposer,
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
		&packages_service.PackageFileInfo{
			Filename: strings.ToLower(fmt.Sprintf("%s.%s.zip", strings.ReplaceAll(cp.Name, "/", "-"), cp.Version)),
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
