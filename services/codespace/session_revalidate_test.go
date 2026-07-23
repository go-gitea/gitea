// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevalidateGatewaySessionAllowsPrivateEndpointAndSSH(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "81818181-8181-4818-8818-818181818181"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     61,
		InteractionGeneration: 7,
		LastActiveUnix:        12,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 61, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": false},
		{"endpoint_id": "public-api", "label": "API", "public": true},
	})))

	endpoint, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionEndpoint,
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.True(t, endpoint.Allowed)

	workspace, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionEndpoint,
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "workspace",
	})
	require.NoError(t, err)
	assert.True(t, workspace.Allowed)

	sshSession, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionSSH,
		UserID:        1,
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	assert.True(t, sshSession.Allowed)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 7, row.InteractionGeneration)
	assert.EqualValues(t, 12, row.LastActiveUnix)
}

func TestRevalidateGatewaySessionDeniesChangedEndpointAndState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "82828282-8282-4828-8828-828282828282"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 62,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 62, []map[string]any{
		{"endpoint_id": "public-api", "label": "API", "public": true},
	})))

	publicEndpoint, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionEndpoint,
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "public-api",
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedEndpointNotFound, publicEndpoint.DeniedCategory)

	wrongUser, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionSSH,
		UserID:        2,
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedPermissionDenied, wrongUser.DeniedCategory)

	activeUUID := "83838383-8383-4838-8838-838383838383"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 activeUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    63,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeUUID, serviceRuntimeMetadataEntry(t, 63, []map[string]any{})))
	active, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionSSH,
		UserID:        1,
		CodespaceUUID: activeUUID,
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedStateUnavailable, active.DeniedCategory)
}
