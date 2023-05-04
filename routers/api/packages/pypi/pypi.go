// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pypi

import (
	"encoding/hex"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	packages_module "code.gitea.io/gitea/modules/packages"
	pypi_module "code.gitea.io/gitea/modules/packages/pypi"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
)

// https://peps.python.org/pep-0426/#name
var (
	normalizer  = strings.NewReplacer(".", "-", "_", "-")
	nameMatcher = regexp.MustCompile(`\A(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\.\-_]*[a-zA-Z0-9])\z`)
)

// https://peps.python.org/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions
var versionMatcher = regexp.MustCompile(`\Av?` +
	`(?:[0-9]+!)?` + // epoch
	`[0-9]+(?:\.[0-9]+)*` + // release segment
	`(?:[-_\.]?(?:a|b|c|rc|alpha|beta|pre|preview)[-_\.]?[0-9]*)?` + // pre-release
	`(?:-[0-9]+|[-_\.]?(?:post|rev|r)[-_\.]?[0-9]*)?` + // post release
	`(?:[-_\.]?dev[-_\.]?[0-9]*)?` + // dev release
	`(?:\+[a-z0-9]+(?:[-_\.][a-z0-9]+)*)?` + // local version
	`\z`)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.Context) {
	packageName := normalizer.Replace(ctx.Params("id"))

	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypePyPI, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// sort package descriptors by version to mimic PyPI format
	sort.Slice(pds, func(i, j int) bool {
		return strings.Compare(pds[i].Version.Version, pds[j].Version.Version) < 0
	})

	ctx.Data["RegistryURL"] = setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/pypi"
	ctx.Data["PackageDescriptor"] = pds[0]
	ctx.Data["PackageDescriptors"] = pds
	ctx.HTML(http.StatusOK, "api/packages/pypi/simple")
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.Context) {
	packageName := normalizer.Replace(ctx.Params("id"))
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypePyPI,
			Name:        packageName,
			Version:     packageVersion,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
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

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.Context) {
	file, fileHeader, err := ctx.Req.FormFile("content")
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

	_, _, hashSHA256, _ := buf.Sums()

	if !strings.EqualFold(ctx.Req.FormValue("sha256_digest"), hex.EncodeToString(hashSHA256)) {
		apiError(ctx, http.StatusBadRequest, "hash mismatch")
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	packageName := normalizer.Replace(ctx.Req.FormValue("name"))
	packageVersion := ctx.Req.FormValue("version")
	if !isValidNameAndVersion(packageName, packageVersion) {
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
				PackageType: packages_model.TypePyPI,
				Name:        packageName,
				Version:     packageVersion,
			},
			SemverCompatible: false,
			Creator:          ctx.Doer,
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
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: fileHeader.Filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusBadRequest, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

func isValidNameAndVersion(packageName, packageVersion string) bool {
	return nameMatcher.MatchString(packageName) && versionMatcher.MatchString(packageVersion)
}
