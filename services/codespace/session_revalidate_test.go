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
	codespaceID := int64(1114)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     61,
		InteractionGeneration: 7,
		LastActiveUnix:        12,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 61, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: false},
		{EndpointId: "public-api", Label: "API", Public: true},
	})))

	endpoint, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, CodespaceId: codespaceID, EndpointId: "app-3000",
		}},
	})
	require.NoError(t, err)
	assert.NotNil(t, endpoint.GetAllowed())

	workspace, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, CodespaceId: codespaceID, EndpointId: "workspace",
		}},
	})
	require.NoError(t, err)
	assert.NotNil(t, workspace.GetAllowed())

	sshSession, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 1, CodespaceId: codespaceID,
		}},
	})
	require.NoError(t, err)
	assert.NotNil(t, sshSession.GetAllowed())

	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 7, row.InteractionGeneration)
	assert.EqualValues(t, 12, row.LastActiveUnix)
}

func TestRevalidateGatewaySessionDeniesChangedEndpointAndState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1115)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 62,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 62, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "public-api", Label: "API", Public: true},
	})))

	publicEndpoint, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, CodespaceId: codespaceID, EndpointId: "public-api",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedEndpointNotFound, publicEndpoint.GetDenied().GetCategory())

	wrongUser, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 2, CodespaceId: codespaceID,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedPermissionDenied, wrongUser.GetDenied().GetCategory())

	activeID := int64(1116)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   activeID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    63,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(activeID, serviceRuntimeMetadataEntry(t, 63, nil)))
	active, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Ssh{Ssh: &codespacev1.SSHSessionBinding{
			UserId: 1, CodespaceId: activeID,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedStateUnavailable, active.GetDenied().GetCategory())
}
