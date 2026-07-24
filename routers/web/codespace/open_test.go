// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	"gitea.dev/modules/templates"
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
		MetadataJSON:       webOpenRuntimeMetadataJSON(t, 91, []map[string]any{{"endpoint_id": "app-3000", "label": "App", "public": false}}),
		MetadataGeneration: 1,
	}))

	ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/"+codespaceUUID+"/open/app-3000")
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
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
		MetadataJSON:       webOpenRuntimeMetadataJSON(t, 92, []map[string]any{{"endpoint_id": "app-3000", "label": "App", "public": true}}),
		MetadataGeneration: 1,
	}))

	ctx, resp := contexttest.MockContext(t, "POST /-/codespaces/"+codespaceUUID+"/open/app-3000")
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
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

func TestOpenEndpointViewIsReadOnly(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "99999999-9999-4999-8999-999999999999"
	insertWebOpenCodespace(t, manager.ID, codespaceUUID, 93)
	require.NoError(t, codespace_service.ReportRuntimeMetadata(t.Context(), manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		MetadataJSON:       webOpenRuntimeMetadataJSON(t, 93, []map[string]any{{"endpoint_id": "app-3000", "label": "App", "public": false}}),
		MetadataGeneration: 1,
	}))

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID+"/open/app-3000", contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
	ctx.SetPathParam("endpoint_id", "app-3000")
	OpenEndpointView(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	info, ok := ctx.Data["OpenEndpoint"].(*codespace_service.OpenEndpointInfo)
	require.True(t, ok)
	require.True(t, info.Available)
	assert.Equal(t, "App", info.Label)
	assert.Equal(t, "https://app-3000-99999999999949998999999999999999.gateway.example.com/", info.TargetURL)
}

func insertWebOpenManager(t *testing.T, gatewayURL string) *codespace_model.Manager {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "manager",
		OwnerID:        0,
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       "[]",
		CreatedUnix:    time.Now().Unix(),
		LastOnlineUnix: time.Now().Unix(),
		MetaJSON:       "{}",
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
		RepoTag:           "default",
		GitProtocol:       codespace_model.GitProtocolHTTP,
		CommitSHA:         "0123456789abcdef0123456789abcdef01234567",
		ManagerID:         managerID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: operationRVersion,
		AutoStopMode:      codespace_model.AutoStopModeDefault,
		CreatedUnix:       1,
		UpdatedUnix:       1,
		LogFilename:       codespaceUUID + ".log",
	}))
}

func webOpenRuntimeMetadataJSON(t *testing.T, operationRVersion int64, endpoints []map[string]any) string {
	t.Helper()
	payload := map[string]any{
		"endpoints": endpoints,
		"boot": map[string]any{
			"operation_rversion": operationRVersion,
			"stage":              "ready",
			"started_unix":       int64(100),
			"last_update_unix":   int64(101),
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(data)
}
