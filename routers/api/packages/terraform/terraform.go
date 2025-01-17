// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/packages"

	"github.com/google/uuid"
)

type TFState struct {
	Version          int             `json:"version"`
	TerraformVersion string          `json:"terraform_version"`
	Serial           uint64          `json:"serial"`
	Lineage          string          `json:"lineage"`
	Outputs          map[string]any  `json:"outputs"`
	Resources        []ResourceState `json:"resources"`
}

type ResourceState struct {
	Mode      string          `json:"mode"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider"`
	Instances []InstanceState `json:"instances"`
}

type InstanceState struct {
	SchemaVersion int            `json:"schema_version"`
	Attributes    map[string]any `json:"attributes"`
}

type LockInfo struct {
	ID      string `json:"id"`
	Created string `json:"created"`
}

var stateLocks = make(map[string]LockInfo)

func apiError(ctx *context.Context, status int, message string) {
	log.Error("Terraform API Error: %d - %s", status, message)
	ctx.JSON(status, map[string]string{"error": message})
}

func getLockID(ctx *context.Context) (string, error) {
	var lock struct {
		ID string `json:"ID"`
	}

	// Read the body of the request and try to parse the JSON
	body, err := io.ReadAll(ctx.Req.Body)
	if err == nil && len(body) > 0 {
		if err := json.Unmarshal(body, &lock); err != nil {
			log.Error("Failed to unmarshal request body: %v", err)
			return "", err
		}
	}

	// We check the presence of lock ID in the request body or request parameters
	if lock.ID == "" {
		lock.ID = ctx.Req.URL.Query().Get("ID")
	}

	if lock.ID == "" {
		apiError(ctx, http.StatusBadRequest, "Missing lock ID")
		return "", fmt.Errorf("missing lock ID")
	}

	log.Info("Extracted lockID: %s", lock.ID)
	return lock.ID, nil
}

func GetState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	log.Info("GetState called for: %s", stateName)

	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:         ctx.Package.Owner.ID,
		Type:            packages_model.TypeTerraform,
		Name:            packages_model.SearchValue{ExactMatch: true, Value: stateName},
		HasFileWithName: stateName,
		IsInternal:      optional.Some(false),
		Sort:            packages_model.SortCreatedDesc,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to fetch latest versions")
		return
	}

	if len(pvs) == 0 {
		apiError(ctx, http.StatusNoContent, "No content available")
		return
	}

	stream, _, _, err := packages.GetFileStreamByPackageNameAndVersion(ctx, &packages.PackageInfo{
		Owner:       ctx.Package.Owner,
		PackageType: packages_model.TypeTerraform,
		Name:        stateName,
		Version:     pvs[0].Version,
	}, &packages.PackageFileInfo{Filename: stateName})
	if err != nil {
		switch {
		case errors.Is(err, packages_model.ErrPackageNotExist):
			apiError(ctx, http.StatusNotFound, "Package not found")
		case errors.Is(err, packages_model.ErrPackageFileNotExist):
			apiError(ctx, http.StatusNotFound, "File not found")
		default:
			apiError(ctx, http.StatusInternalServerError, err.Error())
		}
		return
	}
	defer stream.Close()

	var state TFState
	if err := json.NewDecoder(stream).Decode(&state); err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to parse state file")
		return
	}

	if state.Lineage == "" {
		state.Lineage = uuid.NewString()
		log.Info("Generated new lineage for state: %s", state.Lineage)
	}

	ctx.Resp.Header().Set("Content-Type", "application/json")
	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", stateName))
	ctx.JSON(http.StatusOK, state)
}

func UpdateState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	var newState TFState
	if err := json.Unmarshal(body, &newState); err != nil {
		apiError(ctx, http.StatusBadRequest, "Invalid JSON")
		return
	}

	pvs, _, err := packages_model.SearchLatestVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:         ctx.Package.Owner.ID,
		Type:            packages_model.TypeTerraform,
		Name:            packages_model.SearchValue{ExactMatch: true, Value: stateName},
		HasFileWithName: stateName,
		IsInternal:      optional.Some(false),
		Sort:            packages_model.SortCreatedDesc,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	serial := uint64(0)
	if len(pvs) > 0 {
		if lastSerial, err := strconv.ParseUint(pvs[0].Version, 10, 64); err == nil {
			serial = lastSerial + 1
		}
	}

	packageVersion := fmt.Sprintf("%d", serial)

	packageInfo := &packages.PackageCreationInfo{
		PackageInfo: packages.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeTerraform,
			Name:        stateName,
			Version:     packageVersion,
		},
		Creator:  ctx.Doer,
		Metadata: newState,
	}

	buffer, err := packages_module.CreateHashedBufferFromReader(bytes.NewReader(body))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to create buffer")
		return
	}
	_, _, err = packages.CreatePackageOrAddFileToExisting(ctx, packageInfo, &packages.PackageFileCreationInfo{
		PackageFileInfo: packages.PackageFileInfo{Filename: stateName},
		Creator:         ctx.Doer,
		Data:            buffer,
		IsLead:          true,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to update package")
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{"message": "State updated successfully", "statename": stateName})
}

func LockState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	lockID, err := getLockID(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	// Check if the state is locked
	if lockInfo, locked := stateLocks[stateName]; locked {
		log.Warn("State %s is already locked", stateName)

		// Generate a response for the conflict with information about the current lock
		response := lockInfo // Return full information about the lock
		ctx.JSON(http.StatusConflict, response)
		return
	}

	// Set the lock
	stateLocks[stateName] = LockInfo{
		ID:      lockID,
		Created: time.Now().UTC().Format(time.RFC3339),
	}

	log.Info("Locked state: %s with ID: %s", stateName, lockID)
	ctx.JSON(http.StatusOK, map[string]string{"message": "State locked successfully", "statename": stateName})
}

func UnlockState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	lockID, err := getLockID(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	// Check the lock status
	currentLockInfo, locked := stateLocks[stateName]
	if !locked || currentLockInfo.ID != lockID {
		log.Warn("Unlock attempt failed for state %s with lock ID %s", stateName, lockID)
		apiError(ctx, http.StatusConflict, fmt.Sprintf("State %s is not locked or lock ID mismatch", stateName))
		return
	}

	// Remove the lock
	delete(stateLocks, stateName)
	log.Info("Unlocked state: %s with ID: %s", stateName, lockID)
	ctx.JSON(http.StatusOK, map[string]string{"message": "State unlocked successfully"})
}

func DeleteState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	pvs, err := packages_model.GetVersionsByPackageName(ctx, ctx.Package.Owner.ID, packages_model.TypeTerraform, stateName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to fetch package versions")
		return
	}
	if len(pvs) == 0 {
		ctx.Status(http.StatusNoContent)
		return
	}
	for _, pv := range pvs {
		if err := packages.RemovePackageVersion(ctx, ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, "Failed to delete package version")
			return
		}
	}
	ctx.JSON(http.StatusOK, map[string]string{"message": "State deleted successfully"})
}
