// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenEndpointRedirectsWithOneTimeCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "96969696-9696-4969-8969-969696969696"
	insertWebOpenCodespace(t, manager.ID, codespaceUUID, 91)
	require.NoError(t, codespace_service.ReportRuntimeMetadata(t.Context(), manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           webOpenRuntimeMetadata(t, 91, []map[string]any{{"endpoint_id": "app-3000", "label": "App", "public": false}}),
		MetadataGeneration: 1,
	}))

	ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/"+strconv.FormatInt(webCodespaceIDByUUID(t, codespaceUUID), 10)+"/open/app-3000")
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(webCodespaceIDByUUID(t, codespaceUUID), 10))
	ctx.SetPathParam("endpoint_id", "app-3000")
	OpenEndpoint(ctx)

	require.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", resp.Header().Get("Referrer-Policy"))
	location := resp.Header().Get("Location")
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "app-3000-96969696969649698969969696969696.gateway.example.com", parsed.Host)
	assert.Equal(t, "/.gitea-codespace/open", parsed.Path)
	code := parsed.Query().Get("code")
	require.Regexp(t, `^[0-9a-f]{64}$`, code)
}

func TestOpenEndpointPublicRedirectsWithoutCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "98989898-9898-4989-8989-989898989898"
	insertWebOpenCodespace(t, manager.ID, codespaceUUID, 92)
	require.NoError(t, codespace_service.ReportRuntimeMetadata(t.Context(), manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           webOpenRuntimeMetadata(t, 92, []map[string]any{{"endpoint_id": "app-3000", "label": "App", "public": true}}),
		MetadataGeneration: 1,
	}))

	ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/"+strconv.FormatInt(webCodespaceIDByUUID(t, codespaceUUID), 10)+"/open/app-3000")
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(webCodespaceIDByUUID(t, codespaceUUID), 10))
	ctx.SetPathParam("endpoint_id", "app-3000")
	OpenEndpoint(ctx)

	require.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", resp.Header().Get("Referrer-Policy"))
	location := resp.Header().Get("Location")
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "app-3000-98989898989849898989989898989898.gateway.example.com", parsed.Host)
	assert.Equal(t, "/", parsed.Path)
	assert.Empty(t, parsed.RawQuery)
}

func TestOpenEndpointHidesOtherCreatorCodespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "99999999-9999-4999-8999-999999999999"
	insertWebOpenCodespace(t, manager.ID, codespaceUUID, 93)

	ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/"+strconv.FormatInt(webCodespaceIDByUUID(t, codespaceUUID), 10)+"/open")
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(webCodespaceIDByUUID(t, codespaceUUID), 10))
	Open(ctx)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func insertWebOpenManager(t *testing.T, gatewayURL string) *codespace_model.Manager {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "manager",
		UserID:         0,
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       "[]",
		CreatedUnix:    time.Now().Unix(),
		LastOnlineUnix: time.Now().Unix(),
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.ManagerAddress{
		ManagerID: manager.ID,
		Kind:      codespace_model.ManagerAddressGateway,
		Address:   gatewayURL,
	}))
	return manager
}

func insertWebOpenCodespace(t *testing.T, managerID int64, codespaceUUID string, operationRVersion int64) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:              codespaceUUID,
		UserID:            1,
		RepoID:            2,
		RefType:           "branch",
		RefName:           "main",
		EnvironmentTag:    "default",
		CommitSHA:         "0123456789abcdef0123456789abcdef01234567",
		ManagerID:         managerID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: operationRVersion,
		AutoStopMode:      codespace_model.AutoStopModeDefault,
		CreatedUnix:       1,
		UpdatedUnix:       1,
	}))
}

func webOpenRuntimeMetadata(t *testing.T, operationRVersion int64, endpoints []map[string]any) *codespacev1.RuntimeMetadata {
	t.Helper()
	metadataEndpoints := make([]*codespacev1.RuntimeEndpoint, 0, len(endpoints)+1)
	for _, endpoint := range endpoints {
		metadataEndpoints = append(metadataEndpoints, &codespacev1.RuntimeEndpoint{
			EndpointId: endpoint["endpoint_id"].(string),
			Label:      endpoint["label"].(string),
			Public:     endpoint["public"].(bool),
		})
	}
	metadataEndpoints = append(metadataEndpoints, &codespacev1.RuntimeEndpoint{EndpointId: "workspace", Label: "Workspace"})
	return &codespacev1.RuntimeMetadata{
		Endpoints: metadataEndpoints,
		Boot: &codespacev1.RuntimeBoot{
			OperationRversion: operationRVersion,
			Stage:             codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY,
			StartedUnix:       100,
			LastUpdateUnix:    101,
		},
		ResourceUsage: &codespacev1.RuntimeResourceUsage{
			Cpu:          &codespacev1.RuntimeCPUUsage{UsedMillicores: 125, LimitMillicores: 1000},
			Memory:       &codespacev1.RuntimeMemoryUsage{UsedBytes: 256 * 1024 * 1024, LimitBytes: 1024 * 1024 * 1024},
			Disk:         &codespacev1.RuntimeDiskUsage{UsedBytes: 512 * 1024 * 1024, LimitBytes: 10 * 1024 * 1024 * 1024},
			ObservedUnix: 101,
		},
	}
}
