// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenEndpointTokenAllowsAndConsumes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "91919191-9191-4919-8919-919191919191"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     81,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusQueued,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 5,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 81, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": false},
	})))

	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	require.Len(t, issued.code, 64)
	assert.Equal(t, manager.ID, issued.managerID)
	assert.EqualValues(t, 6, issued.interactionGeneration)
	assert.Regexp(t, `^https://app-3000-91919191919149198919919191919191\.gateway\.example\.com/\.gitea-codespace/open\?code=[0-9a-f]{64}$`, issued.redirectURL)
	assert.True(t, cache.GetCache().IsExist(openTokenCacheKey(issued.code)))

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 6, row.InteractionGeneration)
	assert.Empty(t, row.OperationType)
	assert.Empty(t, row.OperationStatus)

	validated, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	require.NotNil(t, validated.GetAllowed())
	assert.EqualValues(t, 1, validated.GetAllowed().GetUserId())
	assert.Equal(t, codespaceUUID, validated.GetAllowed().GetCodespaceUuid())
	assert.Equal(t, "app-3000", validated.GetAllowed().GetEndpointId())
	assert.EqualValues(t, 7, validated.GetAllowed().GetInteractionGeneration())
	assert.False(t, cache.GetCache().IsExist(openTokenCacheKey(issued.code)))

	again, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, again.GetDenied().GetCategory())
}

func TestOpenEndpointHidesOtherCreatorCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "98979797-9797-4979-8979-979797979797"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 87,
	})

	_, err := OpenEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        2,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "workspace",
	})
	require.ErrorIs(t, err, ErrOpenEndpointNotFound)
}

func TestValidateOpenTokenDeniesAndPreservesTemporarilyInvalidCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "92929292-9292-4929-8929-929292929292"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 82,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 82, []map[string]any{})))
	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "workspace",
	})
	require.NoError(t, err)

	_, err = db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Cols("status").Update(&codespace_model.Codespace{
		Status: codespace_model.StatusStopped,
	})
	require.NoError(t, err)
	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedCodespaceNotRunning, result.GetDenied().GetCategory())
	assert.True(t, cache.GetCache().IsExist(openTokenCacheKey(issued.code)))
}

func TestValidateOpenTokenDeletesExpiredOrMalformedCache(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "93939393-9393-4939-8939-939393939393"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 83,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 83, []map[string]any{})))

	expiredCode := generateOpenTokenCode()
	expiredKey := openTokenCacheKey(expiredCode)
	require.NoError(t, putOpenTokenCacheEntry(expiredKey, openTokenCacheEntry{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "workspace",
		ManagerID:     manager.ID,
		IssuedUnix:    time.Now().Unix() - int64(openTokenExpire/time.Second) - 1,
		ExpiresUnix:   time.Now().Unix() - 1,
	}))
	expired, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: expiredCode})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, expired.GetDenied().GetCategory())
	assert.False(t, cache.GetCache().IsExist(expiredKey))

	malformedCode := generateOpenTokenCode()
	malformedKey := openTokenCacheKey(malformedCode)
	require.NoError(t, cache.GetCache().Put(malformedKey, "{bad", int64(openTokenExpire/time.Second)))
	malformed, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: malformedCode})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, malformed.GetDenied().GetCategory())
	assert.False(t, cache.GetCache().IsExist(malformedKey))
}

func TestValidateOpenTokenEndpointMustRemainPrivate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "94949494-9494-4949-8949-949494949494"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 84,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 84, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": false},
	})))
	issued, err := openEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 84, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
	})))

	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedEndpointNotFound, result.GetDenied().GetCategory())
	assert.True(t, cache.GetCache().IsExist(openTokenCacheKey(issued.code)))
}

func TestValidateOpenTokenVersionExhaustedConsumesCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "95959595-9595-4959-8959-959595959595"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     85,
		InteractionGeneration: math.MaxInt64,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 85, []map[string]any{})))

	code := generateOpenTokenCode()
	key := openTokenCacheKey(code)
	now := time.Now().Unix()
	require.NoError(t, putOpenTokenCacheEntry(key, openTokenCacheEntry{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "workspace",
		ManagerID:     manager.ID,
		IssuedUnix:    now,
		ExpiresUnix:   now + int64(openTokenExpire/time.Second),
	}))
	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: code})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedVersionExhausted, result.GetDenied().GetCategory())
	assert.False(t, cache.GetCache().IsExist(key))
}

func TestOpenEndpointPublicRedirectDoesNotIssueCodeOrAdvance(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "97979797-9797-4979-8979-979797979797"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     86,
		InteractionGeneration: 11,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 86, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
	})))

	result, err := OpenEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	require.True(t, result.Public)
	assert.Equal(t, "https://app-3000-97979797979749798979979797979797.gateway.example.com/", result.RedirectURL)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 11, row.InteractionGeneration)
	assert.Zero(t, row.LastActiveUnix)
}

func insertServiceManagerGatewayAddress(t *testing.T, manager *codespace_model.Manager, gatewayURL string) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerAddress{
		ManagerID: manager.ID,
		Kind:      codespace_model.ManagerAddressGateway,
		Address:   gatewayURL,
	}))
}
