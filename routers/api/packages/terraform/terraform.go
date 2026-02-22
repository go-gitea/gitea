// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
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
	message := helper.ProcessErrorForUser(ctx, status, obj)
	ctx.PlainText(status, message)
}

// GetTerraformState serves the latest version of the state
func GetTerraformState(ctx *context.Context) {
	stateName := ctx.PathParam("name")
	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeTerraform,
		Name:       packages_model.SearchValue{ExactMatch: true, Value: stateName},
		IsInternal: optional.Some(false),
		Sort:       packages_model.SortCreatedDesc,
	})
	if err != nil {
		// TODO: should this be some other fail? When does this error out?
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}
	log.Info("GetTerraformState: %v", pvs[0].Version)
	streamState(ctx, stateName, pvs[0].Version)
}

// GetTerraformStateBySerial serves a specific version of terraform state.
func GetTerraformStateBySerial(ctx *context.Context) {
	streamState(ctx, ctx.PathParam("name"), ctx.PathParam("serial"))
}

// streamState serves the terraform state file
func streamState(ctx *context.Context, name, serial string) {
	s, u, pf, err := packages_service.OpenFileForDownloadByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeTerraform,
			Name:        name,
			Version:     serial,
		},
		&packages_service.PackageFileInfo{
			Filename: "tfstate",
			//			CompositeKey: "state",
		},
		ctx.Req.Method,
	)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) || errors.Is(err, packages_model.ErrPackageFileNotExist) {
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

type TFState struct {
	Version          int    `json:"version"`
	TerraformVersion string `json:"terraform_version"`
	Serial           uint64 `json:"serial"`
}

// UploadState uploads the specific terraform package.
func UploadState(ctx *context.Context) {
	packageName := ctx.PathParam("name")

	if !isValidPackageName(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid package name"))
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

	var state *TFState
	err = json.NewDecoder(buf).Decode(&state)
	if err != nil {
		log.Error("Error decoding json: %v", err)
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeTerraform,
				Name:        packageName,
				Version:     strconv.FormatUint(state.Serial, 10),
			},
			Creator: ctx.Doer,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: "tfstate",
			},
			Creator:           ctx.Doer,
			Data:              buf,
			IsLead:            true,
			OverwriteExisting: true,
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

// DeletePackage deletes the specific terraform package.
func DeletePackage(ctx *context.Context) {
	err := packages_service.RemovePackageVersionByNameAndVersion(
		ctx,
		ctx.Doer,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeTerraform,
			Name:        ctx.PathParam("packagename"),
			//			Version:     ctx.PathParam("filename"),
		},
	)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteState deletes the specific file of a terraform package.
func DeleteState(ctx *context.Context) {
	pv, pf, err := func() (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, ctx.PathParam("packagename"), ctx.PathParam("filename"))
		if err != nil {
			return nil, nil, err
		}

		pf, err := packages_model.GetFileForVersionByName(ctx, pv.ID, "tfstate", packages_model.EmptyFileKey)
		if err != nil {
			return nil, nil, err
		}

		return pv, pf, nil
	}()
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) || errors.Is(err, packages_model.ErrPackageFileNotExist) {
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

func DeleteStateBySerial(ctx *context.Context) {
	serial := ctx.PathParam("serial")
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, ctx.PathParam("name"), serial)
	if err != nil {
		// TODO: check for not exist
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	err = packages_service.DeletePackageVersionAndReferences(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// LockState locks the specific terraform state.
func LockState(ctx *context.Context) {
	packageName := ctx.PathParam("name")

	ok, _, err := globallock.TryLock(ctx, fmt.Sprintf("%d/%s", ctx.Package.Owner.ID, packageName))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		apiError(ctx, http.StatusLocked, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// UnlockState unlock the specific terraform state.
func UnlockState(ctx *context.Context) {
	packageName := ctx.PathParam("name")

	_ = globallock.Unlock(ctx, fmt.Sprintf("%d/%s", ctx.Package.Owner.ID, packageName))

	ctx.Status(http.StatusOK)
}
