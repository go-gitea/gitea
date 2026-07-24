// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRendersCreatorCodespaces(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "22222222-2222-4222-8222-222222222222"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:            codespaceUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces", contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, ctx, 1)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok := ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, codespaceUUID, rows[0].UUID)
	assert.Contains(t, resp.Body.String(), codespaceUUID)
}

func TestRepositoryListRendersCreateFormAndFiltersRows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "23232323-2323-4323-8323-232323232323"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:            codespaceUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	ctx, resp := contexttest.MockContext(t, "GET /user2/repo1/codespaces?ref_type=tag&ref_name=v1.0.0", contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, ctx, 1)
	contexttest.LoadRepo(t, ctx, 2)
	RepositoryList(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok := ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, codespaceUUID, rows[0].UUID)
	assert.Equal(t, "tag", ctx.Data["RefType"])
	assert.Equal(t, "v1.0.0", ctx.Data["RefName"])
	assert.Contains(t, resp.Body.String(), "Create Codespace")
}

func TestDetailRendersCreatorCodespaceNoStore(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "24242424-2424-4424-8424-242424242424"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:            codespaceUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID, contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
	Detail(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	view, ok := ctx.Data["Codespace"].(*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	assert.Equal(t, codespaceUUID, view.UUID)
	assert.Contains(t, resp.Body.String(), "data-refresh-after-ms=\"2000\"")
}

func TestStateRendersFragmentNoStore(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "25252525-2525-4525-8525-252525252525"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:            codespaceUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID+"/state", contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("uuid", codespaceUUID)
	State(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	assert.Contains(t, resp.Body.String(), "id=\"codespace-live-state\"")
	assert.Contains(t, resp.Body.String(), "data-state-url=\"/-/codespaces/"+codespaceUUID+"/state\"")
	assert.Contains(t, resp.Body.String(), "data-refresh-after-ms=\"2000\"")
}

func TestDetailRejectsOtherCreator(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "26262626-2626-4626-8626-262626262626"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:   codespaceUUID,
		Status: codespace_model.StatusStopped,
	})

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+codespaceUUID)
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetPathParam("uuid", codespaceUUID)
	Detail(ctx)

	require.Equal(t, http.StatusForbidden, resp.Code)
	assert.Contains(t, resp.Body.String(), "permission_denied")
}

func insertWebViewCodespace(t *testing.T, codespace *codespace_model.Codespace) {
	t.Helper()
	codespace.UserID = 1
	codespace.RepoID = 2
	codespace.RefType = "branch"
	codespace.RefName = "main"
	codespace.RepoTag = "default"
	codespace.GitProtocol = codespace_model.GitProtocolHTTP
	codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	codespace.CreatedUnix = 1
	codespace.UpdatedUnix = 1
	codespace.LogFilename = codespace.UUID + ".log"
	require.NoError(t, db.Insert(t.Context(), codespace))
}
