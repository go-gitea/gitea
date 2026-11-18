// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
)

type gatewayAccessFailure string

const (
	gatewayAccessManagerOffline      gatewayAccessFailure = "manager_offline"
	gatewayAccessCodespaceNotFound   gatewayAccessFailure = "codespace_not_found"
	gatewayAccessManagerMismatch     gatewayAccessFailure = "manager_mismatch"
	gatewayAccessCodespaceNotRunning gatewayAccessFailure = "codespace_not_running"
	gatewayAccessActiveOperation     gatewayAccessFailure = "active_operation"
	gatewayAccessMetadataRebuilding  gatewayAccessFailure = "metadata_rebuilding"
)

type gatewayRuntimeAccess struct {
	codespace *codespace_model.Codespace
	metadata  runtimeMetadata
}

func loadGatewayRuntimeAccess(ctx context.Context, managerID, codespaceID int64, allowQueuedIdleStop bool) (*gatewayRuntimeAccess, gatewayAccessFailure, error) {
	manager, err := loadCodespaceManager(ctx, managerID)
	if err != nil {
		return nil, "", err
	}
	if manager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(manager) {
		return nil, gatewayAccessManagerOffline, nil
	}

	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceID).Omit("secrets_encrypted").Get(codespace)
	if err != nil {
		return nil, "", err
	}
	if !has {
		return nil, gatewayAccessCodespaceNotFound, nil
	}
	if codespace.ManagerID != managerID {
		return nil, gatewayAccessManagerMismatch, nil
	}
	startup := hasActiveStartup(codespace, time.Now().Unix())
	if codespace.Status != codespace_model.StatusRunning && !startup {
		return nil, gatewayAccessCodespaceNotRunning, nil
	}
	if hasActiveOperation(codespace) && !startup && !(allowQueuedIdleStop && isQueuedIdleStop(codespace)) {
		return nil, gatewayAccessActiveOperation, nil
	}

	entry, hasEntry, err := getRuntimeMetadataEntry(codespaceID)
	if err != nil {
		return nil, "", err
	}
	if !hasEntry || !runtimeMetadataReadyForAccess(codespace, entry.Metadata) {
		return nil, gatewayAccessMetadataRebuilding, nil
	}
	return &gatewayRuntimeAccess{codespace: codespace, metadata: entry.Metadata}, "", nil
}
