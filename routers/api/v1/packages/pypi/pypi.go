// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pypi

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	package_module "code.gitea.io/gitea/modules/packages"
	pypi_module "code.gitea.io/gitea/modules/packages/pypi"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/validation"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	packages_service "code.gitea.io/gitea/services/packages"
)

// https://www.python.org/dev/peps/pep-0503/#normalized-names
var normalizer = strings.NewReplacer(".", "-", "_", "-")
var nameMatcher = regexp.MustCompile(`\A[a-z0-9\.\-_]+\z`)

// https://www.python.org/dev/peps/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions
var versionMatcher = regexp.MustCompile(`^([1-9][0-9]*!)?(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))*((a|b|rc)(0|[1-9][0-9]*))?(\.post(0|[1-9][0-9]*))?(\.dev(0|[1-9][0-9]*))?$`)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.APIContext) {
	packageName := normalizer.Replace(ctx.Params("id"))

	pvs, err := packages.GetVersionsByPackageName(ctx.Package.Owner.ID, packages.TypePyPI, packageName)
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

	ctx.Data["RegistryURL"] = setting.AppURL + "api/v1/packages/" + ctx.Package.Owner.Name + "/pypi"
	ctx.Data["PackageDescriptor"] = pds[0]
	ctx.Data["PackageDescriptors"] = pds
	ctx.Render = templates.HTMLRenderer()
	ctx.HTML(http.StatusOK, "api/packages/pypi/simple")
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	packageName := normalizer.Replace(ctx.Params("id"))
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages.TypePyPI,
			Name:        packageName,
			Version:     packageVersion,
		},
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

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.APIContext) {
	file, fileHeader, err := ctx.Req.FormFile("content")
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	buf, err := package_module.CreateHashedBufferFromReader(file, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	_, _, hashSHA256, _ := buf.Sums()

	if !strings.EqualFold(ctx.Req.FormValue("sha256_digest"), fmt.Sprintf("%x", hashSHA256)) {
		apiError(ctx, http.StatusBadRequest, "hash mismatch")
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	packageName := normalizer.Replace(ctx.Req.FormValue("name"))
	packageVersion := ctx.Req.FormValue("version")
	if !nameMatcher.MatchString(packageName) || !versionMatcher.MatchString(packageVersion) {
		apiError(ctx, http.StatusBadRequest, "invalid name or version")
		return
	}

	projectURL := ctx.Req.FormValue("home_page")
	if !validation.IsValidURL(projectURL) {
		projectURL = ""
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages.TypePyPI,
				Name:        packageName,
				Version:     packageVersion,
			},
			SemverCompatible: true,
			Creator:          ctx.User,
			Metadata: &pypi_module.Metadata{
				Author:          ctx.Req.FormValue("author"),
				Description:     ctx.Req.FormValue("description"),
				LongDescription: ctx.Req.FormValue("long_description"),
				Summary:         ctx.Req.FormValue("summary"),
				ProjectURL:      projectURL,
				License:         ctx.Req.FormValue("license"),
				RequiresPython:  ctx.Req.FormValue("requires_python"),
			},
		},
		&packages_service.PackageFileInfo{
			Filename: fileHeader.Filename,
			Data:     buf,
			IsLead:   true,
		},
	)
	if err != nil {
		if err == packages.ErrDuplicatePackageFile {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusCreated)
}
