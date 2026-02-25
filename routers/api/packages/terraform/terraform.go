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
	"time"
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
	Lineage          string `json:"lineage"`
	// modules are ommited
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

	// Check lineage
	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, packageName)
	if err != nil && !errors.Is(err, packages_model.ErrPackageNotExist) {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if p != nil {
		// Check lock
		props, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, "terraform.lock")
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		if len(props) > 0 && props[0].Value != "" {
			var existingLock LockInfo
			if err := json.Unmarshal([]byte(props[0].Value), &existingLock); err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			if existingLock.ID != ctx.FormString("ID") {
				ctx.Resp.Header().Set("Content-Type", "application/json")
				ctx.Resp.WriteHeader(http.StatusLocked)
				_, _ = ctx.Resp.Write([]byte(props[0].Value))
				return
			}
		}
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	pv, _, err := packages_service.CreatePackageOrAddFileToExisting(
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

	if err := packages_model.InsertOrUpdateProperty(ctx, packages_model.PropertyTypePackage, pv.PackageID, "terraform.lineage", state.Lineage); err != nil {
		log.Error("InsertOrUpdateProperty: %v", err)
	}

	ctx.Status(http.StatusCreated)
}

// DeleteStateBySerial deletes the specific serial of a terraform package as long as it's not the latest one.
func DeleteStateBySerial(ctx *context.Context) {
	serial := ctx.PathParam("serial")
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, ctx.PathParam("name"), serial)
	if errors.Is(err, packages_model.ErrPackageFileNotExist) {
		apiError(ctx, http.StatusNotFound, err)
		return
	} else if err != nil {

		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeTerraform,
		Name:       packages_model.SearchValue{ExactMatch: true, Value: ctx.PathParam("name")},
		IsInternal: optional.Some(false),
		Sort:       packages_model.SortCreatedDesc,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pvs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}
	if pvs[0].ID == pv.ID {
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

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, packageName)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pp, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, "terraform.lock")
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pp) > 0 && pp[0].Value != "" {
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

// LockInfo is the metadata for a terraform lock.
type LockInfo struct {
	ID        string    `json:"ID"`
	Operation string    `json:"Operation"`
	Info      string    `json:"Info"`
	Who       string    `json:"Who"`
	Version   string    `json:"Version"`
	Created   time.Time `json:"Created"`
	Path      string    `json:"Path"`
}

// LockState locks the specific terraform state.
// Internally, it adds a property to the package with the lock information
// Cavieat being that it allocates a package one doesn't exist to attach the property
func LockState(ctx *context.Context) {
	var reqLockInfo LockInfo
	if err := json.NewDecoder(ctx.Req.Body).Decode(&reqLockInfo); err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	packageName := ctx.PathParam("name")
	lockKey := fmt.Sprintf("terraform_lock_%d_%s", ctx.Package.Owner.ID, packageName)

	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, packageName)
	if err != nil {
		// If the package doesn't exist, allocate it for the lock.
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			p = &packages_model.Package{
				OwnerID:   ctx.Package.Owner.ID,
				Type:      packages_model.TypeTerraform,
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

	props, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, "terraform.lock")
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(props) > 0 && props[0].Value != "" {
		apiError(ctx, http.StatusLocked, errors.New("terraform state is already locked"))
		return
	}

	jsonBytes, err := json.Marshal(reqLockInfo)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := packages_model.InsertOrUpdateProperty(ctx, packages_model.PropertyTypePackage, p.ID, "terraform.lock", string(jsonBytes)); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// UnlockState unlock the specific terraform state.
// Internally, it clears the package property
func UnlockState(ctx *context.Context) {
	var reqLockInfo LockInfo
	if err := json.NewDecoder(ctx.Req.Body).Decode(&reqLockInfo); err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	packageName := ctx.PathParam("name")
	lockKey := fmt.Sprintf("terraform_lock_%d_%s", ctx.Package.Owner.ID, packageName)

	release, err := globallock.Lock(ctx, lockKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer release()

	p, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, packageName)
	if err != nil {
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			ctx.Status(http.StatusOK)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	props, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, "terraform.lock")
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// If there are no properties or the property is empty, it should be unlocked
	if len(props) == 0 || props[0].Value == "" {
		ctx.Status(http.StatusOK)
		return
	}

	var existingLock LockInfo
	if err := json.Unmarshal([]byte(props[0].Value), &existingLock); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// we can bypass messing with the lock since it's empty
	if existingLock.ID == "" {
		ctx.Status(http.StatusOK)
		return
	}

	// Unlocking ID must be the same as locker one.
	if existingLock.ID != reqLockInfo.ID {
		apiError(ctx, http.StatusLocked, errors.New("lock ID mismatch"))
		return
	}

	// We can clear the state if lock id matches
	if err := packages_model.InsertOrUpdateProperty(ctx, packages_model.PropertyTypePackage, p.ID, "terraform.lock", ""); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}
