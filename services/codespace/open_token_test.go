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
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueAndValidateOpenTokenAllowsAndConsumes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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

	issued, err := IssueOpenToken(t.Context(), IssueOpenTokenOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	require.Len(t, issued.Code, 64)
	assert.Equal(t, manager.ID, issued.ManagerID)
	assert.EqualValues(t, 6, issued.InteractionGeneration)
	assert.Regexp(t, `^https://app-3000-91919191919149198919919191919191\.gateway\.example\.com/\.gitea-codespace/open\?code=[0-9a-f]{64}$`, issued.RedirectURL)
	assert.True(t, cache.GetCache().IsExist(openTokenCacheKey(issued.Code)))

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 6, row.InteractionGeneration)
	assert.Empty(t, row.OperationType)
	assert.Empty(t, row.OperationStatus)

	validated, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.Code})
	require.NoError(t, err)
	require.True(t, validated.Allowed)
	assert.EqualValues(t, 1, validated.UserID)
	assert.Equal(t, codespaceUUID, validated.CodespaceUUID)
	assert.Equal(t, "app-3000", validated.EndpointID)
	assert.EqualValues(t, 7, validated.InteractionGeneration)
	assert.False(t, cache.GetCache().IsExist(openTokenCacheKey(issued.Code)))

	again, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.Code})
	require.NoError(t, err)
	assert.False(t, again.Allowed)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, again.DeniedCategory)
}

func TestValidateOpenTokenDeniesAndPreservesTemporarilyInvalidCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "92929292-9292-4929-8929-929292929292"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 82,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 82, []map[string]any{})))
	issued, err := IssueOpenToken(t.Context(), IssueOpenTokenOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "workspace",
	})
	require.NoError(t, err)

	_, err = db.GetEngine(t.Context()).ID(codespaceUUID).Cols("status").Update(&codespace_model.Codespace{
		Status: codespace_model.StatusStopped,
	})
	require.NoError(t, err)
	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.Code})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, OpenTokenDeniedCodespaceNotRunning, result.DeniedCategory)
	assert.True(t, cache.GetCache().IsExist(openTokenCacheKey(issued.Code)))
}

func TestValidateOpenTokenDeletesExpiredOrMalformedCache(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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
		IssuedUnix:    time.Now().Unix() - int64(setting.Codespace.OpenTokenExpire/time.Second) - 1,
		ExpiresUnix:   time.Now().Unix() - 1,
	}))
	expired, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: expiredCode})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, expired.DeniedCategory)
	assert.False(t, cache.GetCache().IsExist(expiredKey))

	malformedCode := generateOpenTokenCode()
	malformedKey := openTokenCacheKey(malformedCode)
	require.NoError(t, cache.GetCache().Put(malformedKey, "{bad", int64(setting.Codespace.OpenTokenExpire/time.Second)))
	malformed, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: malformedCode})
	require.NoError(t, err)
	assert.Equal(t, OpenTokenDeniedInvalidCredentials, malformed.DeniedCategory)
	assert.False(t, cache.GetCache().IsExist(malformedKey))
}

func TestValidateOpenTokenEndpointMustRemainPrivate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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
	issued, err := IssueOpenToken(t.Context(), IssueOpenTokenOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 84, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": true},
	})))

	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: issued.Code})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, OpenTokenDeniedEndpointNotFound, result.DeniedCategory)
	assert.True(t, cache.GetCache().IsExist(openTokenCacheKey(issued.Code)))
}

func TestValidateOpenTokenVersionExhaustedConsumesCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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
		ExpiresUnix:   now + int64(setting.Codespace.OpenTokenExpire/time.Second),
	}))
	result, err := ValidateOpenToken(t.Context(), manager, ValidateOpenTokenOptions{Code: code})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, OpenTokenDeniedVersionExhausted, result.DeniedCategory)
	assert.False(t, cache.GetCache().IsExist(key))
}

func TestOpenEndpointPublicRedirectDoesNotIssueCodeOrAdvance(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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

func TestInspectOpenEndpointDoesNotIssueCodeOrAdvance(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	insertServiceManagerGatewayAddress(t, manager, "https://gateway.example.com")
	codespaceUUID := "98989898-9898-4989-8989-989898989898"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     87,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusQueued,
		OperationTrigger:      codespace_model.OperationTriggerIdle,
		OperationCreatedUnix:  time.Now().Unix(),
		InteractionGeneration: 12,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 87, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": false},
	})))

	info, err := InspectOpenEndpoint(t.Context(), OpenEndpointOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		EndpointID:    "app-3000",
	})
	require.NoError(t, err)
	require.True(t, info.Available)
	assert.False(t, info.Public)
	assert.Equal(t, "App", info.Label)
	assert.Equal(t, "https://app-3000-98989898989849898989989898989898.gateway.example.com/", info.TargetURL)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.EqualValues(t, 12, row.InteractionGeneration)
	assert.Equal(t, codespace_model.OperationStop, row.OperationType)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Equal(t, codespace_model.OperationTriggerIdle, row.OperationTrigger)
}

func insertServiceManagerGatewayAddress(t *testing.T, manager *codespace_model.Manager, gatewayURL string) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerAddress{
		ManagerID: manager.ID,
		Kind:      codespace_model.ManagerAddressGateway,
		Address:   gatewayURL,
	}))
}
