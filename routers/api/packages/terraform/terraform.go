// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/routers/api/packages/helper"
	"code.gitea.io/gitea/services/context"
	packages_service "code.gitea.io/gitea/services/packages"
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

var (
	stateStorage = make(map[string]*TFState)
	stateLocks   = make(map[string]string)
	storeMutex   sync.Mutex
)

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		type Error struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		}
		ctx.JSON(status, struct {
			Errors []Error `json:"errors"`
		}{
			Errors: []Error{
				{Status: status, Message: message},
			},
		})
	})
}

func GetState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	log.Info("Function GetState called with parameters: stateName=%s", stateName)

	// Find the package version
	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Type:    packages_model.TypeTerraform,
		Name: packages_model.SearchValue{
			ExactMatch: true,
			Value:      stateName,
		},
		HasFileWithName: stateName,
		IsInternal:      optional.Some(false),
	})
	if err != nil {
		log.Error("Failed to search package versions for state %s: %v", stateName, err)
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// If no version is found, return 204
	if len(pvs) == 0 {
		log.Info("No existing state found for %s, returning 204 No Content", stateName)
		ctx.Resp.WriteHeader(http.StatusNoContent)
		return
	}

	// Get the latest package version
	stateVersion := pvs[0]
	if stateVersion == nil {
		log.Error("State version is nil for state %s", stateName)
		apiError(ctx, http.StatusInternalServerError, "Invalid state version")
		return
	}
	log.Info("Fetching file stream for state %s with version %s", stateName, stateVersion.Version)

	// Log the parameters of GetFileStreamByPackageNameAndVersion call
	log.Info("Fetching file stream with params: Owner=%v, PackageType=%v, Name=%v, Version=%v, Filename=%v",
		ctx.Package.Owner,
		packages_model.TypeTerraform,
		stateName,
		stateVersion.Version,
		stateName,
	)

	// Fetch the file stream
	s, _, _, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeTerraform,
			Name:        stateName,
			Version:     stateVersion.Version,
		},
		&packages_service.PackageFileInfo{
			Filename: stateName,
		},
	)
	if err != nil {
		log.Error("Error fetching file stream for state %s: %v", stateName, err)
		if errors.Is(err, packages_model.ErrPackageNotExist) {
			log.Error("Package does not exist: %v", err)
			apiError(ctx, http.StatusNotFound, "Package not found")
			return
		}
		if errors.Is(err, packages_model.ErrPackageFileNotExist) {
			log.Error("Package file does not exist: %v", err)
			apiError(ctx, http.StatusNotFound, "File not found")
			return
		}
		apiError(ctx, http.StatusInternalServerError, "Failed to fetch file stream")
		return
	}
	defer s.Close()

	// Read the file contents
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, s); err != nil {
		log.Error("Failed to read state file for %s: %v", stateName, err)
		apiError(ctx, http.StatusInternalServerError, "Failed to read state file")
		return
	}

	// Deserialize the state
	var state TFState
	if err := json.Unmarshal(buf.Bytes(), &state); err != nil {
		log.Error("Failed to unmarshal state file for %s: %v", stateName, err)
		apiError(ctx, http.StatusInternalServerError, "Invalid state file format")
		return
	}

	// Ensure lineage is set
	if state.Lineage == "" {
		state.Lineage = uuid.NewString()
		log.Info("Generated new lineage for state %s: %s", stateName, state.Lineage)
	}

	// Send the state in the response
	ctx.Resp.Header().Set("Content-Type", "application/json")
	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", stateName))
	ctx.Resp.WriteHeader(http.StatusOK)
	if _, writeErr := ctx.Resp.Write(buf.Bytes()); writeErr != nil {
		log.Error("Failed to write response for state %s: %v", stateName, writeErr)
	}
}

// UpdateState updates or creates a new Terraform state and interacts with Gitea packages.
func UpdateState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	log.Info("UpdateState called for stateName: %s", stateName)

	storeMutex.Lock()
	defer storeMutex.Unlock()

	// Check for the presence of a lock ID
	requestLockID := ctx.Req.URL.Query().Get("ID")
	if requestLockID == "" {
		apiError(ctx, http.StatusBadRequest, "Missing ID query parameter")
		return
	}

	// Check for blocking state
	if lockID, locked := stateLocks[stateName]; locked && lockID != requestLockID {
		apiError(ctx, http.StatusConflict, fmt.Sprintf("State %s is locked", stateName))
		return
	}

	// Read the request body
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

	// Getting the current serial
	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Type:       packages_model.TypeTerraform,
		Name:       packages_model.SearchValue{ExactMatch: true, Value: stateName},
		IsInternal: optional.Some(false),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to search package versions")
		return
	}

	serial := uint64(1) // Start from 1
	if len(pvs) > 0 {
		lastSerial, _ := strconv.ParseUint(pvs[0].Version, 10, 64)
		serial = lastSerial + 1
	}
	log.Info("State %s updated to serial %d", stateName, serial)

	// Create package information
	packageVersion := fmt.Sprintf("%d", serial)
	packageInfo := &packages_service.PackageCreationInfo{
		PackageInfo: packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeTerraform,
			Name:        stateName,
			Version:     packageVersion,
		},
		SemverCompatible: true,
		Creator:          ctx.Doer,
		Metadata:         newState,
	}

	buffer, err := packages_module.CreateHashedBufferFromReader(bytes.NewReader(body))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to create buffer")
		return
	}

	// Create/update package
	if _, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		packageInfo,
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: stateName,
			},
			Creator: ctx.Doer,
			Data:    buffer,
			IsLead:  true,
		},
	); err != nil {
		apiError(ctx, http.StatusInternalServerError, "Failed to update package")
		return
	}

	log.Info("State %s updated successfully with version %s", stateName, packageVersion)

	ctx.JSON(http.StatusOK, map[string]string{
		"message":   "State updated successfully",
		"statename": stateName,
	})
}

// LockState locks a Terraform state to prevent updates.
func LockState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	log.Info("LockState called for state: %s", stateName)

	// Read the request body
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		log.Error("Failed to read request body: %v", err)
		apiError(ctx, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	// Decode JSON and check lockID
	var lockRequest struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(body, &lockRequest); err != nil || lockRequest.ID == "" {
		log.Error("Invalid lock request body: %v", err)
		apiError(ctx, http.StatusBadRequest, "Invalid or missing lock ID")
		return
	}

	storeMutex.Lock()
	defer storeMutex.Unlock()

	// Check if the state is locked
	if _, locked := stateLocks[stateName]; locked {
		log.Warn("State %s is already locked", stateName)
		apiError(ctx, http.StatusConflict, fmt.Sprintf("State %s is already locked", stateName))
		return
	}

	// Set the lock
	stateLocks[stateName] = lockRequest.ID
	log.Info("State %s locked with ID %s", stateName, lockRequest.ID)

	ctx.JSON(http.StatusOK, map[string]string{
		"message":   "State locked successfully",
		"statename": stateName,
	})
}

// UnlockState unlocks a Terraform state.
func UnlockState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	log.Info("UnlockState called for state: %s", stateName)

	// Extract lockID from request body or parameters
	var unlockRequest struct {
		ID string `json:"ID"`
	}

	// Trying to read the request body
	body, _ := io.ReadAll(ctx.Req.Body)
	if len(body) > 0 {
		_ = json.Unmarshal(body, &unlockRequest) // The error can be ignored, since the ID can also be in the query
	}

	// Check for ID presence
	if unlockRequest.ID == "" {
		log.Error("Missing lock ID in both query and request body")
		apiError(ctx, http.StatusBadRequest, "Missing lock ID")
		return
	}

	log.Info("Extracted lockID: %s", unlockRequest.ID)

	storeMutex.Lock()
	defer storeMutex.Unlock()

	// Check the lock status
	currentLockID, locked := stateLocks[stateName]
	if !locked || currentLockID != unlockRequest.ID {
		log.Warn("Unlock attempt failed for state %s with lock ID %s", stateName, unlockRequest.ID)
		apiError(ctx, http.StatusConflict, fmt.Sprintf("State %s is not locked or lock ID mismatch", stateName))
		return
	}

	// Remove the lock
	delete(stateLocks, stateName)
	log.Info("State %s unlocked successfully", stateName)

	ctx.JSON(http.StatusOK, map[string]string{
		"message":   "State unlocked successfully",
		"statename": stateName,
	})
}

// DeleteState deletes the Terraform state for a given name.
func DeleteState(ctx *context.Context) {
	stateName := ctx.PathParam("statename")
	log.Info("Attempting to delete state: %s", stateName)

	storeMutex.Lock()
	defer storeMutex.Unlock()

	// Check if a state or lock exists
	_, stateExists := stateStorage[stateName]
	_, lockExists := stateLocks[stateName]

	if !stateExists && !lockExists {
		log.Warn("State %s does not exist or is not locked", stateName)
		apiError(ctx, http.StatusNotFound, "State not found")
		return
	}

	// Delete the state and lock
	delete(stateStorage, stateName)
	delete(stateLocks, stateName)

	log.Info("State %s deleted successfully", stateName)
	ctx.JSON(http.StatusOK, map[string]string{
		"message":   "State deleted successfully",
		"statename": stateName,
	})
}
