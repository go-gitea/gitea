// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevalidateGatewaySessionAllowsPrivateEndpointAndSSH(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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

	endpoint, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, RuntimeUuid: codespaceUUID, EndpointId: "app-3000",
		}},
	})
	require.NoError(t, err)
	assert.NotNil(t, endpoint.GetAllowed())

	workspace, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, RuntimeUuid: codespaceUUID, EndpointId: "workspace",
		}},
	})
	require.NoError(t, err)
	assert.NotNil(t, workspace.GetAllowed())

	sshSession, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 1, RuntimeUuid: codespaceUUID,
		}},
	})
	require.NoError(t, err)
	assert.NotNil(t, sshSession.GetAllowed())

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 7, row.InteractionGeneration)
	assert.EqualValues(t, 12, row.LastActiveUnix)
}

func TestRevalidateGatewaySessionDeniesChangedEndpointAndState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "82828282-8282-4828-8828-828282828282"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 62,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 62, []map[string]any{
		{"endpoint_id": "public-api", "label": "API", "public": true},
	})))

	publicEndpoint, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, RuntimeUuid: codespaceUUID, EndpointId: "public-api",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedEndpointNotFound, publicEndpoint.GetDenied().GetCategory())

	wrongUser, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 2, RuntimeUuid: codespaceUUID,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedPermissionDenied, wrongUser.GetDenied().GetCategory())

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
	active, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 1, RuntimeUuid: activeUUID,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedStateUnavailable, active.GetDenied().GetCategory())
}
