// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayAndRuntimeRPCsRejectDisabledCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, true))

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	runningID := int64(1119)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     91,
		InteractionGeneration: 5,
	})
	require.NoError(t, putRuntimeMetadataEntry(runningID, serviceRuntimeMetadataEntry(t, 91, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
		{EndpointId: "private-api", Label: "API", Public: false},
	})))
	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: runningID,
		UserID:      1,
		EndpointID:  "private-api",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 6, loadServiceCodespace(t, runningID).InteractionGeneration)

	restoreEnabled := test.MockVariableValue(&setting.Codespace.Enabled, false)

	publicResult, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceID: runningID,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedStateUnavailable, publicResult.GetDenied().GetCategory())

	sessionResult, err := RevalidateGatewaySession(t.Context(), manager, &codespacev1.RevalidateGatewaySessionRequest{
		Session: &codespacev1.RevalidateGatewaySessionRequest_Endpoint{Endpoint: &codespacev1.EndpointSessionBinding{
			UserId: 1, CodespaceId: runningID, EndpointId: "private-api",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedStateUnavailable, sessionResult.GetDenied().GetCategory())

	sshResult, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceID: runningID,
		PublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedStateUnavailable, sshResult.GetDenied().GetCategory())

	_, err = OpenEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: runningID,
		UserID:      1,
		EndpointID:  "private-api",
	})
	require.ErrorIs(t, err, ErrOpenEndpointUnavailable)

	openResult, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedStateUnavailable, openResult.GetDenied().GetCategory())
	assert.EqualValues(t, 6, loadServiceCodespace(t, runningID).InteractionGeneration)

	creatingID := int64(1121)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    creatingID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     92,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	err = ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceID:        creatingID,
		Metadata:           serviceRuntimeMetadataProto(92, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: 1,
	})
	require.ErrorIs(t, err, ErrRuntimeMetadataStateUnavailable)
	hasReady, err := hasReadyRuntimeMetadataForTest(creatingID, 92)
	require.NoError(t, err)
	assert.False(t, hasReady)

	_, err = requestRuntimeGitSSHKey(t.Context(), manager, RequestRuntimeAccessOptions{
		CodespaceID:       creatingID,
		OperationRVersion: 92,
		GitSSHPublicKey:   servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrRequestRuntimeAccessStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = ?", creatingID)

	restoreEnabled()

	allowedOpen, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.NotNil(t, allowedOpen.GetAllowed())
	assert.EqualValues(t, 7, allowedOpen.GetAllowed().GetInteractionGeneration())
}

func TestDisabledCodespaceRejectsStartupEntrypoints(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	stoppedID := int64(1136)
	runningID := int64(1137)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                stoppedID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 93,
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     94,
		InteractionGeneration: 1,
	})
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	_, err := ResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceID: stoppedID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
	assert.False(t, hasActiveOperation(loadServiceCodespace(t, stoppedID)))

	_, err = ContinueCodespace(t.Context(), ContinueCodespaceOptions{UserID: 1, CodespaceID: runningID})
	require.ErrorIs(t, err, ErrInteractionStateUnavailable)
	assert.EqualValues(t, 1, loadServiceCodespace(t, runningID).InteractionGeneration)
}
