// Copyright 2026 The Gitea Authors. All rights reserved.
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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	packages_module "code.gitea.io/gitea/modules/packages"
	terraform_module "code.gitea.io/gitea/modules/packages/terraform"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
)

var packageNameRegex = regexp.MustCompile(`\A[-_+.\w]+\z`)

const (
	stateFilename = "tfstate"
)

func apiError(ctx *context.Context, status int, obj any) {
	message := helper.ProcessErrorForUser(ctx, status, obj)
	ctx.PlainText(status, message)
}

// GetTerraformState serves the latest version of the state
func GetTerraformState(ctx *context.Context) {
	stateName := ctx.PathParam("name")
	pv, err := getLatestVersion(ctx, stateName)
	if errors.Is(err, packages_model.ErrPackageNotExist) {
		apiError(ctx, http.StatusNotFound, nil)
		return
	} else if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	streamState(ctx, stateName, pv.Version)
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
			PackageType: packages_model.TypeTerraformState,
			Name:        name,
			Version:     serial,
		},
		&packages_service.PackageFileInfo{
			Filename: stateFilename,
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

// UploadState uploads the specific terraform package.
func UploadState(ctx *context.Context) {
	packageName := ctx.PathParam("name")

	if !isValidPackageName(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid package name"))
		return
	}
	lockKey := getLockKey(ctx)
	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformState, packageName)
	if err != nil && !errors.Is(err, packages_model.ErrPackageNotExist) {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if p != nil {
		// Check lock
		lock, err := terraform_module.GetLock(ctx, p.ID)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		// If the state is locked, enforce the lock
		if lock.IsLocked() && lock.ID != ctx.FormString("ID") {
			ctx.JSON(http.StatusLocked, lock)
			return
		}
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

	state, err := terraform_module.ParseState(buf)
	if err != nil {
		log.Error("Error decoding state: %v", err)
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
				PackageType: packages_model.TypeTerraformState,
				Name:        packageName,
				Version:     strconv.FormatUint(state.Serial, 10),
			},
			Creator: ctx.Doer,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: stateFilename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, packages_model.ErrDuplicatePackageFile):
			apiError(ctx, http.StatusConflict, err)
		case errors.Is(err, packages_service.ErrQuotaTotalCount), errors.Is(err, packages_service.ErrQuotaTypeSize), errors.Is(err, packages_service.ErrQuotaTotalSize):
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

// DeleteStateBySerial deletes the specific serial of a terraform package as long as it's not the latest one.
func DeleteStateBySerial(ctx *context.Context) {
	lockKey := getLockKey(ctx)
	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	serial := ctx.PathParam("serial")
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformState, ctx.PathParam("name"), serial)
	if errors.Is(err, packages_model.ErrPackageNotExist) {
		apiError(ctx, http.StatusNotFound, err)
		return
	} else if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pvLatest, err := getLatestVersion(ctx, ctx.PathParam("name"))
	if errors.Is(err, packages_model.ErrPackageNotExist) {
		apiError(ctx, http.StatusNotFound, err)
		return
	} else if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if pvLatest.ID == pv.ID {
		apiError(ctx, http.StatusForbidden, errors.New("cannot delete the latest version"))
		return
	}

	err = packages_service.DeletePackageVersionAndReferences(ctx, pv)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// DeleteState deletes the specific file of a terraform package.
// Fails if the state is locked
func DeleteState(ctx *context.Context) {
	packageName := ctx.PathParam("name")

	lockKey := getLockKey(ctx)
	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformState, packageName)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	lock, err := terraform_module.GetLock(ctx, p.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if lock.IsLocked() {
		apiError(ctx, http.StatusLocked, errors.New("terraform state is locked"))
		return
	}

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  p.ID,
		IsInternal: optional.None[bool](),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	err = packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypePackage, p.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	for _, pv := range pvs {
		if err := packages_service.RemovePackageVersion(ctx, ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	if err := packages_model.DeletePackageByID(ctx, p.ID); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// LockState locks the specific terraform state.
// Internally, it adds a property to the package with the lock information
// Caveat being that it allocates a package if one doesn't exist to attach the property
func LockState(ctx *context.Context) {
	packageName := ctx.PathParam("name")
	if !isValidPackageName(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid package name"))
		return
	}

	var reqLockInfo *terraform_module.LockInfo
	reqLockInfo, err := terraform_module.ParseLockInfo(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	lockKey := getLockKey(ctx)
	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformState, packageName)
	if err != nil {
		// If the package doesn't exist, allocate it for the lock.
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			p = &packages_model.Package{
				OwnerID:   ctx.Package.Owner.ID,
				Type:      packages_model.TypeTerraformState,
				Name:      packageName,
				LowerName: strings.ToLower(packageName),
			}
			if p, err = packages_model.TryInsertPackage(ctx, p); err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	currentLock, err := terraform_module.GetLock(ctx, p.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if currentLock.IsLocked() {
		ctx.JSON(http.StatusLocked, currentLock)
		return
	}

	err = terraform_module.SetLock(ctx, p.ID, reqLockInfo)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// UnlockState unlock the specific terraform state.
// Internally, it clears the package property
func UnlockState(ctx *context.Context) {
	packageName := ctx.PathParam("name")
	if !isValidPackageName(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("invalid package name"))
		return
	}

	reqLockInfo, err := terraform_module.ParseLockInfo(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	lockKey := getLockKey(ctx)
	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraformState, packageName)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			ctx.Status(http.StatusOK)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	existingLock, err := terraform_module.GetLock(ctx, p.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// we can bypass messing with the lock since it's empty
	if !existingLock.IsLocked() {
		ctx.Status(http.StatusOK)
		return
	}

	// Unlocking ID must be the same as locker one.
	if existingLock.ID != reqLockInfo.ID {
		apiError(ctx, http.StatusLocked, errors.New("lock ID mismatch"))
		return
	}
	// We can clear the state if lock id matches
	err = terraform_module.RemoveLock(ctx, p.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

func getLatestVersion(ctx *context.Context, packageName string) (*packages_model.PackageVersion, error) {
	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeTerraformState,
		Name:       packages_model.SearchValue{ExactMatch: true, Value: packageName},
		IsInternal: optional.Some(false),
		Sort:       packages_model.SortCreatedDesc,
	})
	if err != nil {
		return nil, err
	}
	if len(pvs) == 0 {
		return nil, packages_model.ErrPackageNotExist
	}
	return pvs[0], nil
}

func getLockKey(ctx *context.Context) string {
	return fmt.Sprintf("terraform_lock_%d_%s", ctx.Package.Owner.ID, strings.ToLower(ctx.PathParam("name")))
}
