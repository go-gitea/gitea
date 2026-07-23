// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"strconv"
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

func TestLogsReturnsJSONPage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "92909090-9090-4090-8090-909090909090"
	insertWebLogCodespace(t, manager.ID, codespaceUUID, 94)
	_, err := codespace_service.UpdateLog(t.Context(), manager, codespace_service.UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 94,
		Offset:            0,
		Lines: []codespace_service.LogLine{
			{TimestampUnixNano: time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC).UnixNano(), Message: "first"},
			{TimestampUnixNano: time.Date(2026, 7, 22, 3, 0, 1, 0, time.UTC).UnixNano(), Message: "second"},
		},
	})
	require.NoError(t, err)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID+"/logs?offset=0&limit=40")
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
	Logs(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	var body codespace_service.ReadLogResult
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.EqualValues(t, 0, body.Offset)
	assert.Positive(t, body.NextOffset)
	assert.False(t, body.EOF)
	assert.True(t, body.Truncated)
	require.Len(t, body.Lines, 1)
	assert.Contains(t, body.Lines[0], "first\n")
}

func TestLogsRejectsInvalidArgument(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "93909090-9090-4090-8090-909090909090"
	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID+"/logs?offset=bad")
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
	Logs(ctx)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	assert.Equal(t, "invalid_argument", decodeLogError(t, resp.Body.Bytes()).Category)
}

func TestLogsReportsOffsetConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "94909090-9090-4090-8090-909090909090"
	insertWebLogCodespace(t, manager.ID, codespaceUUID, 95)
	result, err := codespace_service.UpdateLog(t.Context(), manager, codespace_service.UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 95,
		Offset:            0,
		Lines: []codespace_service.LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 3, 1, 0, 0, time.UTC).UnixNano(),
			Message:           "first",
		}},
	})
	require.NoError(t, err)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID+"/logs?offset="+strconv.FormatInt(result.NextOffset+1, 10))
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
	Logs(ctx)

	require.Equal(t, http.StatusConflict, resp.Code)
	errBody := decodeLogError(t, resp.Body.Bytes())
	assert.Equal(t, "offset_conflict", errBody.Category)
	assert.Equal(t, result.NextOffset, errBody.CurrentOffset)
}

func TestLogPreviewAndDownloadUseStoredContent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "95909090-9090-4090-8090-909090909090"
	insertWebLogCodespace(t, manager.ID, codespaceUUID, 96)
	_, err := codespace_service.UpdateLog(t.Context(), manager, codespace_service.UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 96,
		Offset:            0,
		Lines: []codespace_service.LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 3, 2, 0, 0, time.UTC).UnixNano(),
			Message:           "download same",
		}},
	})
	require.NoError(t, err)

	detailCtx, detailResp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID, contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, detailCtx, 1)
	detailCtx.SetPathParam("uuid", codespaceUUID)
	Detail(detailCtx)
	require.Equal(t, http.StatusOK, detailResp.Code)
	assert.Contains(t, detailResp.Body.String(), "download same")
	assert.Contains(t, detailResp.Body.String(), "/-/codespaces/"+codespaceUUID+"/logs/download")

	downloadCtx, downloadResp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID+"/logs/download")
	contexttest.LoadUser(t, downloadCtx, 1)
	downloadCtx.SetPathParam("uuid", codespaceUUID)
	DownloadLogs(downloadCtx)
	require.Equal(t, http.StatusOK, downloadResp.Code)
	assert.Equal(t, "no-store", downloadResp.Header().Get("Cache-Control"))
	assert.Contains(t, downloadResp.Header().Get("Content-Disposition"), codespaceUUID+".log")
	assert.Contains(t, downloadResp.Body.String(), "download same\n")
}

func insertWebLogCodespace(t *testing.T, managerID int64, codespaceUUID string, operationRVersion int64) {
	t.Helper()
	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		UserID:                1,
		RepoID:                2,
		RefType:               "branch",
		RefName:               "main",
		RepoTag:               "default",
		GitProtocol:           codespace_model.GitProtocolHTTP,
		CommitSHA:             "0123456789abcdef0123456789abcdef01234567",
		ManagerID:             managerID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     operationRVersion,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
		AutoStopMode:          codespace_model.AutoStopModeDefault,
		CreatedUnix:           1,
		UpdatedUnix:           1,
		LogFilename:           codespaceUUID + ".log",
	}))
}

func decodeLogError(t *testing.T, data []byte) logErrorResponse {
	t.Helper()
	var body logErrorResponse
	require.NoError(t, json.Unmarshal(data, &body))
	return body
}
