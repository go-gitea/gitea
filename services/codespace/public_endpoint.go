// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"fmt"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
)

const (
	// PublicEndpointDeniedInvalidEndpoint means the endpoint id is not a public endpoint id.
	PublicEndpointDeniedInvalidEndpoint = "invalid_endpoint"
	// PublicEndpointDeniedCodespaceNotFound means the Codespace no longer exists.
	PublicEndpointDeniedCodespaceNotFound = "codespace_not_found"
	// PublicEndpointDeniedManagerMismatch means the Codespace is bound to another Manager.
	PublicEndpointDeniedManagerMismatch = "manager_mismatch"
	// PublicEndpointDeniedManagerOffline means the Manager is not online.
	PublicEndpointDeniedManagerOffline = "manager_offline"
	// PublicEndpointDeniedStateUnavailable means the lifecycle state cannot serve public traffic.
	PublicEndpointDeniedStateUnavailable = "state_unavailable"
	// PublicEndpointDeniedActiveOperation means a lifecycle operation is active.
	PublicEndpointDeniedActiveOperation = "active_operation"
	// PublicEndpointDeniedMetadataRebuilding means Runtime Metadata is absent or not ready.
	PublicEndpointDeniedMetadataRebuilding = "metadata_rebuilding"
	// PublicEndpointDeniedEndpointNotPublic means the endpoint is absent or private.
	PublicEndpointDeniedEndpointNotPublic = "endpoint_not_public"
)

// ValidatePublicEndpointOptions identifies one public Endpoint authorization request.
type ValidatePublicEndpointOptions struct {
	CodespaceUUID string
	EndpointID    string
}

// ValidatePublicEndpointResult contains the public Endpoint authorization result.
type ValidatePublicEndpointResult struct {
	Allowed        bool
	DeniedCategory string
}

// ValidatePublicEndpoint authorizes unauthenticated traffic to a public Runtime Endpoint.
func ValidatePublicEndpoint(ctx context.Context, manager *codespace_model.Manager, opts ValidatePublicEndpointOptions) (*ValidatePublicEndpointResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, fmt.Errorf("manager is required")
	}
	if !setting.Codespace.Enabled {
		return denyPublicEndpoint(PublicEndpointDeniedStateUnavailable), nil
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if !validPublicEndpointID(opts.EndpointID) {
		return denyPublicEndpoint(PublicEndpointDeniedInvalidEndpoint), nil
	}
	currentManager, err := loadFetchManager(ctx, manager.ID)
	if err != nil {
		return nil, err
	}
	if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
		return denyPublicEndpoint(PublicEndpointDeniedManagerOffline), nil
	}

	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return denyPublicEndpoint(PublicEndpointDeniedCodespaceNotFound), nil
	}
	if codespace.ManagerID != manager.ID {
		return denyPublicEndpoint(PublicEndpointDeniedManagerMismatch), nil
	}
	if codespace.Status != codespace_model.StatusRunning {
		return denyPublicEndpoint(PublicEndpointDeniedStateUnavailable), nil
	}
	if hasActiveOperation(codespace) {
		return denyPublicEndpoint(PublicEndpointDeniedActiveOperation), nil
	}

	entry, hasEntry, err := getRuntimeMetadataEntry(opts.CodespaceUUID)
	if err != nil {
		return nil, err
	}
	if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
		return denyPublicEndpoint(PublicEndpointDeniedMetadataRebuilding), nil
	}
	for _, endpoint := range entry.Metadata.Endpoints {
		if endpoint.EndpointID == opts.EndpointID {
			if endpoint.Public {
				return &ValidatePublicEndpointResult{Allowed: true}, nil
			}
			return denyPublicEndpoint(PublicEndpointDeniedEndpointNotPublic), nil
		}
	}
	return denyPublicEndpoint(PublicEndpointDeniedEndpointNotPublic), nil
}

func validPublicEndpointID(endpointID string) bool {
	return endpointID != "workspace" && endpointIDPattern.MatchString(endpointID)
}

func runtimeMetadataReadyForRunning(codespace *codespace_model.Codespace, metadata runtimeMetadata) bool {
	return metadata.Boot.Stage == bootStageReady &&
		metadata.Boot.OperationRVersion <= codespace.OperationRVersion
}

func denyPublicEndpoint(category string) *ValidatePublicEndpointResult {
	return &ValidatePublicEndpointResult{DeniedCategory: category}
}
