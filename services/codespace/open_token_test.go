// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenEndpointTokenAllowsAndConsumes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1119)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     81,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 5,
	})
	entry := serviceRuntimeMetadataEntry(t, 81, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: false},
	})
	entry.Metadata.Endpoints[0].URL += "project%20path?code=old&next=old"
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, entry))
	manager.GatewaySSHAddr = "new-ssh.example.com:22"
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("gateway_ssh_addr").Update(manager)
	require.NoError(t, err)

	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: codespaceID,
		UserID:      1,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	require.Len(t, issued.code, 64)
	assert.Equal(t, manager.ID, issued.managerID)
	assert.EqualValues(t, 6, issued.interactionGeneration)
	assert.Regexp(t, `^https://app-3000\.gateway\.example\.com/\.gitea-codespace/open\?code=[0-9a-f]{64}$`, issued.redirectURL)
	assertOpenTokenExists(t, issued.code, true)

	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 6, row.InteractionGeneration)
	assert.False(t, hasActiveOperation(row))

	validated, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	require.NotNil(t, validated.GetAllowed())
	assert.EqualValues(t, 1, validated.GetAllowed().GetUserId())
	assert.Equal(t, codespaceID, validated.GetAllowed().GetCodespaceId())
	assert.Equal(t, "app-3000", validated.GetAllowed().GetEndpointId())
	assert.EqualValues(t, 7, validated.GetAllowed().GetInteractionGeneration())
	assertOpenTokenExists(t, issued.code, false)

	again, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, again.GetDenied().GetCategory())

	setServiceManagerGatewayURL(t, manager, "https://new-gateway.example.com")
	_, err = OpenEndpoint(t.Context(), OpenEndpointOptions{CodespaceID: codespaceID, UserID: 1, EndpointID: "app-3000"})
	require.ErrorIs(t, err, ErrOpenEndpointUnavailable)
}

func TestOpenEndpointHidesOtherCreatorCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1120)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 87,
	})

	_, err := OpenEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: codespaceID,
		UserID:      2,
		EndpointID:  "workspace",
	})
	require.ErrorIs(t, err, ErrOpenEndpointNotFound)
}

func TestOpenEndpointDuringInitialization(t *testing.T) {
	for _, status := range []string{codespace_model.StatusCreating, codespace_model.StatusStopped} {
		t.Run(status, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			manager := insertServiceManager(t)
			markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
			setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
			cs := &codespace_model.Codespace{
				ID: int64(1119), Status: status, OperationRVersion: 81,
				OperationTrigger: codespace_model.OperationTriggerUser, OperationCreatedUnix: time.Now().Unix(),
				OperationStartedUnix: time.Now().Unix(), OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
			}
			insertServiceCodespace(t, manager.ID, cs)
			opts := OpenEndpointOptions{UserID: cs.UserID, CodespaceID: cs.ID, EndpointID: workspaceEndpointID}
			require.NoError(t, putRuntimeMetadataEntry(cs.ID, serviceRuntimeMetadataEntry(t, 80, nil)))
			_, err := openEndpoint(t.Context(), opts)
			require.ErrorIs(t, err, ErrOpenEndpointUnavailable)
			require.NoError(t, putRuntimeMetadataEntry(cs.ID, serviceRuntimeMetadataEntry(t, 81, nil)))
			issued, err := openEndpoint(t.Context(), opts)
			require.NoError(t, err)
			validated, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
			require.NoError(t, err)
			require.NotNil(t, validated.GetAllowed())
			_, failure, err := loadGatewayRuntimeAccess(t.Context(), manager.ID, cs.ID, false)
			require.NoError(t, err)
			require.Empty(t, failure)
			view := &CreatorCodespaceView{}
			applyCreatorDisplayState(cs, view, manager, true)
			applyCreatorActions(cs, view, manager)
			require.True(t, view.CanOpen)
			require.Contains(t, []string{DisplayBooting, DisplayResuming}, view.DisplayStatus)
			_, err = db.GetEngine(t.Context()).ID(cs.ID).Cols("operation_deadline_unix").Update(&codespace_model.Codespace{OperationDeadlineUnix: time.Now().Add(-time.Second).Unix()})
			require.NoError(t, err)
			_, err = openEndpoint(t.Context(), opts)
			require.ErrorIs(t, err, ErrOpenEndpointUnavailable)
		})
	}
}

func TestValidateOpenTokenDeniesAndPreservesTemporarilyInvalidCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1121)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 82,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 82, nil)))
	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: codespaceID,
		UserID:      1,
		EndpointID:  "workspace",
	})
	require.NoError(t, err)

	_, err = db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols("status").Update(&codespace_model.Codespace{
		Status: codespace_model.StatusStopped,
	})
	require.NoError(t, err)
	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedCodespaceNotRunning, result.GetDenied().GetCategory())
	assertOpenTokenExists(t, issued.code, true)
}

func TestValidateOpenTokenDeletesExpiredCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1122)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 83,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 83, nil)))

	expiredCode := generateOpenTokenCode()
	insertOpenToken(t, expiredCode, codespaceID, "workspace", manager.ID, time.Now().Unix()-1)
	expired, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: expiredCode})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, expired.GetDenied().GetCategory())
	assertOpenTokenExists(t, expiredCode, false)
}

func TestValidateOpenTokenEndpointMustRemainPrivate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1123)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 84,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 84, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: false},
	})))
	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: codespaceID,
		UserID:      1,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 84, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
	})))

	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedEndpointNotFound, result.GetDenied().GetCategory())
	assertOpenTokenExists(t, issued.code, true)
}

func TestValidateOpenTokenVersionExhaustedConsumesCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1124)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     85,
		InteractionGeneration: math.MaxInt64,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 85, nil)))

	code := generateOpenTokenCode()
	now := time.Now().Unix()
	insertOpenToken(t, code, codespaceID, "workspace", manager.ID, now+int64(openTokenExpire/time.Second))
	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedVersionExhausted, result.GetDenied().GetCategory())
	assertOpenTokenExists(t, code, false)
}

func TestOpenEndpointPublicRedirectDoesNotIssueCodeOrAdvance(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	setServiceManagerGatewayURL(t, manager, "https://gateway.example.com")
	codespaceID := int64(1125)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     86,
		InteractionGeneration: 11,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 86, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "app-3000", Label: "App", Public: true},
	})))

	result, err := OpenEndpoint(t.Context(), OpenEndpointOptions{
		CodespaceID: codespaceID,
		UserID:      1,
		EndpointID:  "app-3000",
	})
	require.NoError(t, err)
	require.True(t, result.Public)
	assert.Equal(t, "https://app-3000.gateway.example.com/", result.RedirectURL)
	row := loadServiceCodespace(t, codespaceID)
	assert.EqualValues(t, 11, row.InteractionGeneration)
	assert.Zero(t, row.LastActiveUnix)
}

func setServiceManagerGatewayURL(t *testing.T, manager *codespace_model.Manager, gatewayURL string) {
	t.Helper()
	manager.GatewayURL = gatewayURL
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("gateway_url").Update(manager)
	require.NoError(t, err)
}

func insertOpenToken(t *testing.T, code string, codespaceID int64, endpointID string, managerID, expiresUnix int64) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.OpenToken{
		CodespaceID: codespaceID,
		CodeHash:    openTokenCodeHash(code),
		UserID:      1,
		ManagerID:   managerID,
		EndpointID:  endpointID,
		ExpiresUnix: expiresUnix,
	}))
}

func assertOpenTokenExists(t *testing.T, code string, expected bool) {
	t.Helper()
	exists, err := db.GetEngine(t.Context()).ID(openTokenCodeHash(code)).Exist(new(codespace_model.OpenToken))
	require.NoError(t, err)
	assert.Equal(t, expected, exists)
}
