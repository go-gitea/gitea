// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"

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

// RevalidateSessionKind identifies the Gateway session binding shape.
type RevalidateSessionKind string

const (
	// RevalidateSessionEndpoint checks an authenticated HTTP or WebSocket Endpoint session.
	RevalidateSessionEndpoint RevalidateSessionKind = "endpoint"
	// RevalidateSessionSSH checks an existing Gateway SSH session.
	RevalidateSessionSSH RevalidateSessionKind = "ssh"
)

// RevalidateGatewaySessionOptions contains one existing Gateway session binding.
type RevalidateGatewaySessionOptions struct {
	Kind          RevalidateSessionKind
	UserID        int64
	CodespaceUUID string
	EndpointID    string
}

// RevalidateGatewaySessionResult contains the mutually exclusive session revalidation result.
type RevalidateGatewaySessionResult struct {
	Allowed        bool
	DeniedCategory string
}

// RevalidateGatewaySession checks whether an existing Gateway session remains authorized.
func RevalidateGatewaySession(ctx context.Context, manager *codespace_model.Manager, opts RevalidateGatewaySessionOptions) (*RevalidateGatewaySessionResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if !setting.Codespace.Enabled {
		return denyGatewaySession(SessionDeniedStateUnavailable), nil
	}
	if err := validateRevalidateGatewaySessionOptions(opts); err != nil {
		return nil, err
	}

	currentManager, err := loadFetchManager(ctx, manager.ID)
	if err != nil {
		return nil, err
	}
	if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
		return denyGatewaySession(SessionDeniedStateUnavailable), nil
	}

	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return denyGatewaySession(SessionDeniedCodespaceNotFound), nil
	}
	if codespace.ManagerID != manager.ID {
		return denyGatewaySession(SessionDeniedManagerMismatch), nil
	}
	if opts.UserID != codespace.UserID {
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
	canUseGateway, err := userCanUseGatewayAccess(ctx, user)
	if err != nil {
		return nil, err
	}
	if !canUseGateway {
		return denyGatewaySession(SessionDeniedLoginRestricted), nil
	}

	entry, hasEntry, err := getRuntimeMetadataEntry(opts.CodespaceUUID)
	if err != nil {
		return nil, err
	}
	if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
		return denyGatewaySession(SessionDeniedMetadataRebuilding), nil
	}

	switch opts.Kind {
	case RevalidateSessionSSH:
		return &RevalidateGatewaySessionResult{Allowed: true}, nil
	case RevalidateSessionEndpoint:
		if opts.EndpointID == "workspace" || privateEndpointExists(entry.Metadata, opts.EndpointID) {
			return &RevalidateGatewaySessionResult{Allowed: true}, nil
		}
		return denyGatewaySession(SessionDeniedEndpointNotFound), nil
	default:
		return nil, fmt.Errorf("unsupported session kind %q", opts.Kind)
	}
}

func validateRevalidateGatewaySessionOptions(opts RevalidateGatewaySessionOptions) error {
	if opts.UserID <= 0 {
		return errors.New("user_id must be positive")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return err
	}
	switch opts.Kind {
	case RevalidateSessionSSH:
		if opts.EndpointID != "" {
			return errors.New("ssh session must not include endpoint_id")
		}
	case RevalidateSessionEndpoint:
		if opts.EndpointID != "workspace" && !endpointIDPattern.MatchString(opts.EndpointID) {
			return errors.New("invalid endpoint_id")
		}
	default:
		return errors.New("session is required")
	}
	return nil
}

func privateEndpointExists(metadata runtimeMetadata, endpointID string) bool {
	for _, endpoint := range metadata.Endpoints {
		if endpoint.EndpointID == endpointID {
			return !endpoint.Public
		}
	}
	return false
}

func denyGatewaySession(category string) *RevalidateGatewaySessionResult {
	return &RevalidateGatewaySessionResult{DeniedCategory: category}
}
