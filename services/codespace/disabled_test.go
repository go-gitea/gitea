// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

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
	markServiceManagerOnline(t, manager, `["default"]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	insertServiceUserSSHKey(t, 1, testGitSSHPublicKey)

	runningUUID := "91919191-9191-4919-8919-919191919191"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     91,
		InteractionGeneration: 5,
	})
	require.NoError(t, putRuntimeMetadataEntry(runningUUID, serviceRuntimeMetadataEntry(t, 91, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
		{"endpoint_id": "private-api", "label": "API", "public": false},
	})))
	issued, err := IssueOpenToken(t.Context(), IssueOpenTokenOptions{
		UserID:        1,
		CodespaceUUID: runningUUID,
		EndpointID:    "private-api",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 6, loadServiceCodespace(t, runningUUID).InteractionGeneration)

	restoreEnabled := test.MockVariableValue(&setting.Codespace.Enabled, false)

	publicResult, err := ValidatePublicEndpoint(t.Context(), manager, ValidatePublicEndpointOptions{
		CodespaceUUID: runningUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	assert.Equal(t, PublicEndpointDeniedStateUnavailable, publicResult.DeniedCategory)

	sessionResult, err := RevalidateGatewaySession(t.Context(), manager, RevalidateGatewaySessionOptions{
		Kind:          RevalidateSessionEndpoint,
		UserID:        1,
		CodespaceUUID: runningUUID,
		EndpointID:    "private-api",
	})
	require.NoError(t, err)
	assert.Equal(t, SessionDeniedStateUnavailable, sessionResult.DeniedCategory)

	sshResult, err := VerifySSHPublicKey(t.Context(), manager, VerifySSHPublicKeyOptions{
		CodespaceUUID: runningUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.NoError(t, err)
	assert.Equal(t, SSHAuthDeniedStateUnavailable, sshResult.DeniedCategory)

	openInfo, err := InspectOpenEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        1,
		CodespaceUUID: runningUUID,
		EndpointID:    "private-api",
	})
	require.NoError(t, err)
	assert.False(t, openInfo.Available)
	assert.Equal(t, OpenTokenDeniedStateUnavailable, openInfo.NotAvailableCategory)

	_, err = IssueOpenToken(t.Context(), IssueOpenTokenOptions{
		UserID:        1,
		CodespaceUUID: runningUUID,
		EndpointID:    "private-api",
	})
	require.ErrorIs(t, err, ErrOpenEndpointUnavailable)

	openResult, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.Code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedStateUnavailable, openResult.DeniedCategory)
	assert.EqualValues(t, 6, loadServiceCodespace(t, runningUUID).InteractionGeneration)

	creatingUUID := "92929292-9292-4929-8929-929292929292"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  creatingUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     92,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	err = ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      creatingUUID,
		MetadataJSON:       serviceRuntimeMetadataJSON(t, 92, "ready", "Workspace"),
		MetadataGeneration: 1,
	})
	require.ErrorIs(t, err, ErrRuntimeMetadataStateUnavailable)
	hasReady, err := HasReadyRuntimeMetadata(t.Context(), creatingUUID, 92)
	require.NoError(t, err)
	assert.False(t, hasReady)

	_, err = EnsureGitSSHKey(t.Context(), manager, EnsureGitSSHKeyOptions{
		CodespaceUUID: creatingUUID,
		PublicKey:     servicePublicKeyWire(t, testGitSSHPublicKey),
	})
	require.ErrorIs(t, err, ErrEnsureGitSSHKeyStateUnavailable)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", creatingUUID)

	restoreEnabled()

	allowedOpen, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.Code})
	require.NoError(t, err)
	assert.True(t, allowedOpen.Allowed)
	assert.EqualValues(t, 7, allowedOpen.InteractionGeneration)
}

func TestDisabledCodespaceRejectsStartupEntrypoints(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	stoppedUUID := "93939393-9393-4939-8939-939393939391"
	runningUUID := "93939393-9393-4939-8939-939393939392"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              stoppedUUID,
		Status:            codespace_model.StatusStopped,
		OperationRVersion: 93,
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     94,
		InteractionGeneration: 1,
	})
	token, err := GetOrCreateRegistrationToken(t.Context(), ManagerSettingsOptions{Scope: ManagerSettingsScopeSite})
	require.NoError(t, err)

	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	_, err = ResumeCodespace(t.Context(), LifecycleActionOptions{UserID: 1, CodespaceUUID: stoppedUUID})
	require.ErrorIs(t, err, ErrLifecycleActionStateUnavailable)
	assert.Empty(t, loadServiceCodespace(t, stoppedUUID).OperationType)

	_, err = ContinueCodespace(t.Context(), ContinueCodespaceOptions{UserID: 1, CodespaceUUID: runningUUID})
	require.ErrorIs(t, err, ErrInteractionStateUnavailable)
	assert.EqualValues(t, 1, loadServiceCodespace(t, runningUUID).InteractionGeneration)

	_, _, err = RegisterManager(t.Context(), token)
	require.ErrorIs(t, err, ErrRegistrationStateUnavailable)
}
