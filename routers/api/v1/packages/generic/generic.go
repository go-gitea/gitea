// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package generic

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/util/filebuffer"

	package_service "code.gitea.io/gitea/services/packages"
)

var packageNameRegex = regexp.MustCompile(`\A[A-Za-z0-9\.\_\-\+]+\z`)
var packageVersionRegex = regexp.MustCompile(`\A(?:\.?[\w\+-]+\.?)+\z`)
var filenameRegex = packageNameRegex

// DownloadPackage serves the specific generic package.
func DownloadPackage(ctx *context.APIContext) {
	packageName, packageVersion, filename, err := sanitizeParameters(ctx)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	p, err := models.GetPackageByNameAndVersion(ctx.Repo.Repository.ID, models.PackageGeneric, packageName, packageVersion)
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		log.Error("Error getting package: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}

	pfs, err := p.GetFiles()
	if err != nil {
		log.Error("Error getting package files: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}

	pf := pfs[0]

	if !strings.EqualFold(pf.LowerName, filename) {
		ctx.Error(http.StatusNotFound, "", models.ErrPackageNotExist)
		return
	}

	packageStore := packages.NewContentStore()
	s, err := packageStore.Get(pf.ID)
	if err != nil {
		log.Error("Error reading package file: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackage uploads the specific generic package.
// Duplicated packages get rejected.
func UploadPackage(ctx *context.APIContext) {
	packageName, packageVersion, filename, err := sanitizeParameters(ctx)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	defer ctx.Req.Body.Close()
	r, err := filebuffer.CreateFromReader(ctx.Req.Body, 32*1024*1024)
	if err != nil {
		log.Error("Error in CreateFromReader: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}
	defer r.Close()

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		models.PackageGeneric,
		packageName,
		packageVersion,
		nil,
		false,
	)
	if err != nil {
		if err == models.ErrDuplicatePackage {
			ctx.Error(http.StatusBadRequest, "", err)
			return
		}
		log.Error("Error in CreatePackage: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}

	_, err = package_service.AddFileToPackage(p, filename, r.Size(), r)
	if err != nil {
		log.Error("Error in AddFileToPackage: %v", err)
		if err := models.DeletePackageByID(p.ID); err != nil {
			log.Error("Error in DeletePackageByID: %v", err)
		}
		ctx.Error(http.StatusInternalServerError, "", "")
	}

	ctx.PlainText(http.StatusCreated, nil)
}

// DeletePackage deletes the specific generic package.
func DeletePackage(ctx *context.APIContext) {
	packageName, packageVersion, _, err := sanitizeParameters(ctx)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	err = package_service.DeletePackage(ctx.Repo.Repository, models.PackageGeneric, packageName, packageVersion)
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", "")
	}
}

func sanitizeParameters(ctx *context.APIContext) (packageName, packageVersion, filename string, err error) {
	packageName = ctx.Params("packagename")
	packageVersion = ctx.Params("packageversion")
	filename = ctx.Params("filename")

	if !packageNameRegex.MatchString(packageName) || !packageVersionRegex.MatchString(packageVersion) || !filenameRegex.MatchString(filename) {
		err = errors.New("Invalid package name, package version or filename")
	}
	return
}
