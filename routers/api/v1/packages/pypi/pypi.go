// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pypi

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	pypi_module "code.gitea.io/gitea/modules/packages/pypi"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"

	package_service "code.gitea.io/gitea/services/packages"
)

// https://www.python.org/dev/peps/pep-0503/#normalized-names
var normalizer = strings.NewReplacer(".", "-", "_", "-")
var nameMatcher = regexp.MustCompile(`\A[a-z0-9\.\-_]+\z`)

// https://www.python.org/dev/peps/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions
var versionMatcher = regexp.MustCompile(`^([1-9][0-9]*!)?(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))*((a|b|rc)(0|[1-9][0-9]*))?(\.post(0|[1-9][0-9]*))?(\.dev(0|[1-9][0-9]*))?$`)

// PackageMetadata returns the metadata for a single package
func PackageMetadata(ctx *context.APIContext) {
	packageName := normalizer.Replace(ctx.Params("id"))

	packages, err := models.GetPackagesByName(ctx.Repo.Repository.ID, models.PackagePyPI, packageName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	if len(packages) == 0 {
		ctx.Error(http.StatusNotFound, "", err)
		return
	}

	pypiPackages, err := intializePackages(packages)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	ctx.Data["RegistryURL"] = setting.AppURL + "api/v1/repos/" + ctx.Repo.Repository.FullName() + "/packages/pypi"
	ctx.Data["Package"] = pypiPackages[0]
	ctx.Data["Packages"] = pypiPackages
	ctx.Render = templates.HTMLRenderer()
	ctx.HTML(http.StatusOK, "api/packages/pypi/simple")
}

// DownloadPackageContent serves the content of a package
func DownloadPackageContent(ctx *context.APIContext) {
	packageName := normalizer.Replace(ctx.Params("id"))
	packageVersion := ctx.Params("version")
	filename := ctx.Params("filename")

	s, pf, err := package_service.GetFileStreamByPackageNameAndVersion(ctx.Repo.Repository, models.PackagePyPI, packageName, packageVersion, filename)
	if err != nil {
		if err == models.ErrPackageNotExist || err == models.ErrPackageFileNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.APIContext) {
	file, fileHeader, err := ctx.Req.FormFile("content")
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}
	defer file.Close()

	h256 := sha256.New()
	if _, err := io.Copy(h256, file); err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	if !strings.EqualFold(ctx.Req.FormValue("sha256_digest"), fmt.Sprintf("%x", h256.Sum(nil))) {
		ctx.Error(http.StatusBadRequest, "", "hash mismatch")
		return
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	packageName := normalizer.Replace(ctx.Req.FormValue("name"))
	packageVersion := ctx.Req.FormValue("version")
	if !nameMatcher.MatchString(packageName) || !versionMatcher.MatchString(packageVersion) {
		ctx.Error(http.StatusBadRequest, "", "invalid name or version")
		return
	}

	metadata := &pypi_module.Metadata{
		Author:         ctx.Req.FormValue("author"),
		Description:    ctx.Req.FormValue("description"),
		Summary:        ctx.Req.FormValue("summary"),
		ProjectURL:     ctx.Req.FormValue("home_page"),
		License:        ctx.Req.FormValue("license"),
		RequiresPython: ctx.Req.FormValue("requires_python"),
	}

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		models.PackagePyPI,
		packageName,
		packageVersion,
		metadata,
		true,
	)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	_, err = package_service.AddFileToPackage(p, fileHeader.Filename, fileHeader.Size, file)
	if err != nil {
		if err == models.ErrDuplicatePackageFile {
			ctx.Error(http.StatusBadRequest, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}
