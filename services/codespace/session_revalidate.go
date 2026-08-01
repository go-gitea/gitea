// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
)

const (
	// SessionDeniedCodespaceNotFound means the Codespace no longer exists.
	SessionDeniedCodespaceNotFound = "codespace_not_found"
	// SessionDeniedCodespaceNotRunning means the Codespace is not running.
	SessionDeniedCodespaceNotRunning = "codespace_not_running"
	// SessionDeniedManagerMismatch means the Codespace is bound to another Manager.
	SessionDeniedManagerMismatch = "manager_mismatch"
	// SessionDeniedPermissionDenied means the session user does not match the Codespace creator.
	SessionDeniedPermissionDenied = "permission_denied"
	// SessionDeniedLoginRestricted means the Codespace creator cannot currently log in.
	SessionDeniedLoginRestricted = "login_restricted"
	// SessionDeniedStateUnavailable means the lifecycle state cannot keep the session.
	SessionDeniedStateUnavailable = "state_unavailable"
	// SessionDeniedMetadataRebuilding means Runtime Metadata is absent or not ready.
	SessionDeniedMetadataRebuilding = "metadata_rebuilding"
	// SessionDeniedEndpointNotFound means the authenticated Endpoint binding is no longer private.
	SessionDeniedEndpointNotFound = "endpoint_not_found"
)

// RevalidateGatewaySession checks whether an existing Gateway session remains authorized.
func RevalidateGatewaySession(ctx context.Context, manager *codespace_model.Manager, request *codespacev1.RevalidateGatewaySessionRequest) (*codespacev1.RevalidateGatewaySessionResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if !setting.Codespace.Enabled {
		return denyGatewaySession(SessionDeniedStateUnavailable), nil
	}
	userID, codespaceUUID, endpointID, sshSession, err := validateGatewaySessionBinding(request)
	if err != nil {
		return nil, err
	}

	currentManager, err := loadCodespaceManager(ctx, manager.ID)
	if err != nil {
		return nil, err
	}
	if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
		return denyGatewaySession(SessionDeniedStateUnavailable), nil
	}

	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return denyGatewaySession(SessionDeniedCodespaceNotFound), nil
	}
	if codespace.ManagerID != manager.ID {
		return denyGatewaySession(SessionDeniedManagerMismatch), nil
	}
	if userID != codespace.UserID {
		return denyGatewaySession(SessionDeniedPermissionDenied), nil
	}
	if codespace.Status != codespace_model.StatusRunning {
		return denyGatewaySession(SessionDeniedCodespaceNotRunning), nil
	}
	if hasActiveOperation(codespace) {
		return denyGatewaySession(SessionDeniedStateUnavailable), nil
	}

	user, err := user_model.GetUserByID(ctx, codespace.UserID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			return denyGatewaySession(SessionDeniedLoginRestricted), nil
		}
		return nil, err
	}
	canUseGateway, err := codespaceUserCanLogIn(ctx, user)
	if err != nil {
		return nil, err
	}
	if !canUseGateway {
		return denyGatewaySession(SessionDeniedLoginRestricted), nil
	}

	entry, hasEntry, err := getRuntimeMetadataEntry(codespaceUUID)
	if err != nil {
		return nil, err
	}
	if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
		return denyGatewaySession(SessionDeniedMetadataRebuilding), nil
	}

	endpoint, endpointFound := entry.Metadata.endpointByID(endpointID)
	if sshSession || (endpointFound && !endpoint.Public) {
		return &codespacev1.RevalidateGatewaySessionResponse{
			Outcome: &codespacev1.RevalidateGatewaySessionResponse_Allowed{Allowed: &codespacev1.SessionAllowed{}},
		}, nil
	}
	return denyGatewaySession(SessionDeniedEndpointNotFound), nil
}

func validateGatewaySessionBinding(request *codespacev1.RevalidateGatewaySessionRequest) (userID int64, codespaceUUID, endpointID string, sshSession bool, err error) {
	if endpoint := request.GetEndpoint(); endpoint != nil {
		userID = endpoint.GetUserId()
		codespaceUUID = endpoint.GetCodespaceUuid()
		endpointID = endpoint.GetEndpointId()
		if endpointID != workspaceEndpointID && !endpointIDPattern.MatchString(endpointID) {
			return 0, "", "", false, errors.New("invalid endpoint_id")
		}
	} else if ssh := request.GetSsh(); ssh != nil {
		userID = ssh.GetUserId()
		codespaceUUID = ssh.GetCodespaceUuid()
		sshSession = true
	} else {
		return 0, "", "", false, errors.New("session is required")
	}
	if userID <= 0 {
		return 0, "", "", false, errors.New("user_id must be positive")
	}
	if err := codespace_model.ValidateUUID(codespaceUUID); err != nil {
		return 0, "", "", false, err
	}
	return userID, codespaceUUID, endpointID, sshSession, nil
}

func denyGatewaySession(category string) *codespacev1.RevalidateGatewaySessionResponse {
	return &codespacev1.RevalidateGatewaySessionResponse{
		Outcome: &codespacev1.RevalidateGatewaySessionResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: category},
		},
	}
}
