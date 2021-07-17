// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package generic

import (
	"errors"
	"net/http"
	"regexp"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util/filebuffer"

	package_service "code.gitea.io/gitea/services/packages"

	"github.com/hashicorp/go-version"
)

var packageNameRegex = regexp.MustCompile(`\A[A-Za-z0-9\.\_\-\+]+\z`)
var filenameRegex = packageNameRegex

// DownloadPackageContent serves the specific generic package.
func DownloadPackageContent(ctx *context.APIContext) {
	packageName, packageVersion, filename, err := sanitizeParameters(ctx)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	s, pf, err := package_service.GetPackageFileStream(ctx.Repo.Repository, models.PackageGeneric, packageName, packageVersion, filename)
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

// UploadPackage uploads the specific generic package.
// Duplicated packages get rejected.
func UploadPackage(ctx *context.APIContext) {
	packageName, packageVersion, filename, err := sanitizeParameters(ctx)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}

	upload, close, err := ctx.UploadStream()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := filebuffer.CreateFromReader(upload, 32*1024*1024)
	if err != nil {
		log.Error("Error creating file buffer: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}
	defer buf.Close()

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
		log.Error("Error creating package: %v", err)
		ctx.Error(http.StatusInternalServerError, "", "")
		return
	}

	_, err = package_service.AddFileToPackage(p, filename, buf.Size(), buf)
	if err != nil {
		log.Error("Error adding file to package: %v", err)
		if err := models.DeletePackageByID(p.ID); err != nil {
			log.Error("Error deleting package by id: %v", err)
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

func sanitizeParameters(ctx *context.APIContext) (string, string, string, error) {
	packageName := ctx.Params("packagename")
	filename := ctx.Params("filename")

	if !packageNameRegex.MatchString(packageName) || !filenameRegex.MatchString(filename) {
		return "", "", "", errors.New("Invalid package name or filename")
	}

	v, err := version.NewSemver(ctx.Params("packageversion"))
	if err != nil {
		return "", "", "", err
	}

	return packageName, v.String(), filename, nil
}
