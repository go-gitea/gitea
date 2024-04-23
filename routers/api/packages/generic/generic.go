// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package generic

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
)

var (
	packageNameRegex = regexp.MustCompile(`\A[-_+.\w]+\z`)
	filenameRegex    = regexp.MustCompile(`\A[-_+=:;.()\[\]{}~!@#$%^& \w]+\z`)
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

// DownloadPackageFile serves the specific generic package.
func DownloadPackageFile(ctx *context.Context) {
	s, u, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeGeneric,
			Name:        ctx.Params("packagename"),
			Version:     ctx.Params("packageversion"),
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

	helper.ServePackageFile(ctx, s, u, pf)
}

func isValidPackageName(packageName string) bool {
	if len(packageName) == 1 && !unicode.IsLetter(rune(packageName[0])) && !unicode.IsNumber(rune(packageName[0])) {
		return false
	}
	return packageNameRegex.MatchString(packageName) && packageName != ".."
}

func isValidFileName(filename string) bool {
	return filenameRegex.MatchString(filename) &&
		strings.TrimSpace(filename) == filename &&
		filename != "." && filename != ".."
}

// UploadPackage uploads the specific generic package.
// Duplicated packages get rejected.
func UploadPackage(ctx *context.Context) {
	packageName := ctx.Params("packagename")
	filename := ctx.Params("filename")

	if !isValidPackageName(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid package name"))
		return
	}

	if !isValidFileName(filename) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid filename"))
		return
	}

	packageVersion := ctx.Params("packageversion")
	if packageVersion != strings.TrimSpace(packageVersion) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid package version"))
		return
	}

	upload, needToClose, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if needToClose {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		log.Error("Error creating hashed buffer: %v", err)
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeGeneric,
				Name:        packageName,
				Version:     packageVersion,
			},
			Creator: ctx.Doer,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

// DeletePackage deletes the specific generic package.
func DeletePackage(ctx *context.Context) {
	err := packages_service.RemovePackageVersionByNameAndVersion(
		ctx,
		ctx.Doer,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeGeneric,
			Name:        ctx.Params("packagename"),
			Version:     ctx.Params("packageversion"),
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeletePackageFile deletes the specific file of a generic package.
func DeletePackageFile(ctx *context.Context) {
	pv, pf, err := func() (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeGeneric, ctx.Params("packagename"), ctx.Params("packageversion"))
		if err != nil {
			return nil, nil, err
		}

		pf, err := packages_model.GetFileForVersionByName(ctx, pv.ID, ctx.Params("filename"), packages_model.EmptyFileKey)
		if err != nil {
			return nil, nil, err
		}

		return pv, pf, nil
	}()
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pfs, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pfs) == 1 {
		if err := packages_service.RemovePackageVersion(ctx, ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	} else {
		if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.Status(http.StatusNoContent)
}
