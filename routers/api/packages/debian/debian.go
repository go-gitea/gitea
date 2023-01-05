// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
)

var (
	namePattern    = regexp.MustCompile(`\A[a-z0-9][a-z0-9\+\-\.]+\z`)
	versionPattern = regexp.MustCompile(`\A([0-9]:)?[a-zA-Z0-9\.\+\~]+(-[a-zA-Z0-9\.\+\~])?\z`) // TODO: hypens should be allowed if revision is present
	archPattern    = regexp.MustCompile(`\A[a-z0-9\-]+\z`)
)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetPackage(ctx *context.Context) {
	// packageName := ctx.Params("packagename")
	// packageVersion := ctx.Params("packageversion")
	// packageArch := ctx.Params("arch")
	// filename := fmt.Sprintf("%s_%s_%s.deb", packageName, packageVersion, packageArch)
	filename := ctx.Params("filename")
	log.Info("Filename: %s", filename)

	splitter := regexp.MustCompile(`^([^_]+)_([^_]+)_([^.]+).deb$`)
	matches := splitter.FindStringSubmatch(filename)
	if matches == nil {
		apiError(ctx, http.StatusBadRequest, "Invalid filename")
		return
	}
	packageName := matches[1]
	packageVersion := matches[2]

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeDebian,
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

func PutPackage(ctx *context.Context) {
	packageName := ctx.Params("packagename")

	if !namePattern.MatchString(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("Invalid package name"))
		return
	}

	packageVersion := ctx.Params("packageversion")
	if packageVersion != strings.TrimSpace(packageVersion) {
		apiError(ctx, http.StatusBadRequest, errors.New("Invalid package version"))
		return
	}

	packageArch := ctx.Params("arch")
	// TODO Check arch

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload, 32*1024*1024)
	if err != nil {
		log.Error("Error creating hashed buffer: %v", err)
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	filename := fmt.Sprintf("%s_%s_%s.deb", packageName, packageVersion, packageArch)
	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeDebian,
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

func DeletePackage(ctx *context.Context) {
	packageName := ctx.Params("packagename")
	packageVersion := ctx.Params("packageversion")
	packageArch := ctx.Params("arch")
	filename := fmt.Sprintf("%s_%s_%s.deb", packageName, packageVersion, packageArch)

	pv, pf, err := func() (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeDebian, packageName, packageVersion)
		if err != nil {
			return nil, nil, err
		}

		pf, err := packages_model.GetFileForVersionByName(ctx, pv.ID, filename, packages_model.EmptyFileKey)
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
		if err := packages_service.RemovePackageVersion(ctx.Doer, pv); err != nil {
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
