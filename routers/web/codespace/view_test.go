// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"strconv"
	"testing"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFiltersCurrentCreatorByRepositoryOwner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	otherCreatorUUID := "39393939-3939-4939-8939-393939393939"
	insertWebViewCodespace(t, &codespace_model.Codespace{UUID: otherCreatorUUID, Status: codespace_model.StatusStopped})

	personalRepositoryUUID := "42424242-4242-4242-8242-424242424242"
	insertWebViewCodespace(t, &codespace_model.Codespace{UUID: personalRepositoryUUID, Status: codespace_model.StatusStopped})
	_, err := db.GetEngine(t.Context()).Where("uuid = ?", personalRepositoryUUID).Cols("user_id").Update(&codespace_model.Codespace{UserID: 2})
	require.NoError(t, err)

	organizationRepositoryUUID := "43434343-4343-4343-8343-434343434343"
	insertWebViewCodespace(t, &codespace_model.Codespace{UUID: organizationRepositoryUUID, Status: codespace_model.StatusStopped})
	_, err = db.GetEngine(t.Context()).Where("uuid = ?", organizationRepositoryUUID).Cols("user_id", "repo_id").Update(&codespace_model.Codespace{UserID: 2, RepoID: 3})
	require.NoError(t, err)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-all-repositories")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok := ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 2)
	assert.ElementsMatch(t, []string{personalRepositoryUUID, organizationRepositoryUUID}, []string{rows[0].UUID, rows[1].UUID})
	for _, row := range rows {
		assert.Contains(t, resp.Body.String(), "codespace-row-"+strconv.FormatInt(row.ID, 10))
	}
	assert.Contains(t, resp.Body.String(), "codespace.all_repositories")
	assert.Nil(t, ctx.Data["CodespaceRepositoryOwner"])
	assert.NotNil(t, ctx.Data["Page"])

	ctx, resp = contexttest.MockContext(t, "GET /-/codespaces?owner=USER2&partial=true", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-personal-repositories")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok = ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, personalRepositoryUUID, rows[0].UUID)
	repositoryOwner, ok := ctx.Data["CodespaceRepositoryOwner"].(*user_model.User)
	require.True(t, ok)
	assert.EqualValues(t, 2, repositoryOwner.ID)
	assert.Equal(t, "/-/codespaces?owner=user2&partial=true", ctx.Data["CodespaceListStateURL"])

	ctx, resp = contexttest.MockContext(t, "GET /-/codespaces?owner=org3&partial=true", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-org-list")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok = ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, organizationRepositoryUUID, rows[0].UUID)
	repositoryOwner, ok = ctx.Data["CodespaceRepositoryOwner"].(*user_model.User)
	require.True(t, ok)
	assert.EqualValues(t, 3, repositoryOwner.ID)
	assert.Equal(t, "/-/codespaces?owner=org3&partial=true", ctx.Data["CodespaceListStateURL"])
	pager, ok := ctx.Data["Page"].(*context.Pagination)
	require.True(t, ok)
	assert.Equal(t, "owner=org3", string(pager.GetParams()))
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
}

func TestListRendersEmptyRepositoryOwner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces?owner=org17", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-empty-owner")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "codespace.empty_repository_owner_desc:org17")
}

func TestDetailRendersCreatorCodespaceNoStore(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "24242424-2424-4424-8424-242424242424"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:   codespaceUUID,
		Status: codespace_model.StatusCreating,
	})
	codespaceID := webCodespaceIDByUUID(t, codespaceUUID)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+strconv.FormatInt(codespaceID, 10), contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-detail")})
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(codespaceID, 10))
	Detail(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	view, ok := ctx.Data["Codespace"].(*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	assert.Equal(t, codespaceUUID, view.UUID)
	assert.Equal(t, codespace_service.DetailModeLogs, ctx.Data["CodespaceTab"])
	explicit, ok := ctx.Data["CodespaceTabExplicit"].(bool)
	require.True(t, ok)
	assert.False(t, explicit)
}

func TestDetailPreservesExplicitOverviewTab(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "30303030-3030-4030-8030-303030303030"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:   codespaceUUID,
		Status: codespace_model.StatusCreating,
	})
	codespaceID := webCodespaceIDByUUID(t, codespaceUUID)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+strconv.FormatInt(codespaceID, 10)+"?tab=overview", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-overview")})
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(codespaceID, 10))
	Detail(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, codespace_service.DetailModeOverview, ctx.Data["CodespaceTab"])
	explicit, ok := ctx.Data["CodespaceTabExplicit"].(bool)
	require.True(t, ok)
	assert.True(t, explicit)
	assert.NotContains(t, resp.Body.String(), "data-log-next-offset=\"0\"")
}

func TestDetailOpensGatewayRecoveryModal(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	codespaceUUID := "27272727-2727-4727-8727-272727272727"
	insertWebOpenCodespace(t, manager.ID, codespaceUUID, 94)
	codespaceID := webCodespaceIDByUUID(t, codespaceUUID)
	require.NoError(t, codespace_service.ReportRuntimeMetadata(t.Context(), manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceUUID: codespaceUUID,
		Metadata: webOpenRuntimeMetadata(t, 94, []*codespacev1.RuntimeEndpoint{{
			EndpointId: "app-3000",
			Label:      "App",
		}}),
		MetadataGeneration: 1,
	}))

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+strconv.FormatInt(codespaceID, 10)+"?open_endpoint=app-3000", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-open")})
	contexttest.LoadUser(t, ctx, 1)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(codespaceID, 10))
	Detail(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	modal, ok := ctx.Data["OpenEndpointModal"].(*openEndpointModalData)
	require.True(t, ok)
	assert.Equal(t, "App", modal.Label)
	assert.Equal(t, "codespace.authenticated_endpoint", modal.Access)
	assert.Equal(t, "/-/codespaces/"+strconv.FormatInt(codespaceID, 10)+"/open/app-3000", modal.OpenPath)
}

func TestDetailRejectsOtherCreator(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "26262626-2626-4626-8626-262626262626"
	insertWebViewCodespace(t, &codespace_model.Codespace{
		UUID:   codespaceUUID,
		Status: codespace_model.StatusStopped,
	})
	codespaceID := webCodespaceIDByUUID(t, codespaceUUID)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+strconv.FormatInt(codespaceID, 10))
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetPathParam("codespace_id", strconv.FormatInt(codespaceID, 10))
	Detail(ctx)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func insertWebViewCodespace(t *testing.T, codespace *codespace_model.Codespace) {
	t.Helper()
	codespace.UserID = 1
	codespace.RepoID = 2
	codespace.RefType = "branch"
	codespace.RefName = "main"
	codespace.EnvironmentTag = "default"
	codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	codespace.CreatedUnix = 1
	codespace.UpdatedUnix = 1
	var err error
	if codespace.UUID == "" {
		_, err = db.GetEngine(t.Context()).Table(codespace).Omit("uuid").Insert(codespace)
	} else {
		err = db.Insert(t.Context(), codespace)
	}
	require.NoError(t, err)
}

func webCodespaceIDByUUID(t *testing.T, codespaceUUID string) int64 {
	t.Helper()
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Get(codespace)
	require.NoError(t, err)
	require.True(t, has)
	return codespace.ID
}
