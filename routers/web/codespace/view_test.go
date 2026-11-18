// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"net/http"
	"net/url"
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

	otherCreatorID := int64(1000)
	insertWebViewCodespace(t, &codespace_model.Codespace{ID: otherCreatorID, Status: codespace_model.StatusStopped})

	personalRepositoryID := int64(1001)
	insertWebViewCodespace(t, &codespace_model.Codespace{ID: personalRepositoryID, Status: codespace_model.StatusStopped})
	_, err := db.GetEngine(t.Context()).Where("id = ?", personalRepositoryID).Cols("user_id").Update(&codespace_model.Codespace{UserID: 2})
	require.NoError(t, err)

	organizationRepositoryID := int64(1002)
	insertWebViewCodespace(t, &codespace_model.Codespace{ID: organizationRepositoryID, Status: codespace_model.StatusStopped})
	_, err = db.GetEngine(t.Context()).Where("id = ?", organizationRepositoryID).Cols("user_id", "repo_id").Update(&codespace_model.Codespace{UserID: 2, RepoID: 3})
	require.NoError(t, err)

	ctx, resp := contexttest.MockContext(t, "GET /-/codespaces", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-all-repositories")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok := ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 2)
	assert.ElementsMatch(t, []int64{personalRepositoryID, organizationRepositoryID}, []int64{rows[0].ID, rows[1].ID})
	assert.Nil(t, ctx.Data["CodespaceRepositoryOwner"])
	assert.NotNil(t, ctx.Data["Page"])

	ctx, resp = contexttest.MockContext(t, "GET /-/codespaces?owner=USER2&partial=true", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-personal-repositories")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok = ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, personalRepositoryID, rows[0].ID)
	repositoryOwner, ok := ctx.Data["CodespaceRepositoryOwner"].(*user_model.User)
	require.True(t, ok)
	assert.EqualValues(t, 2, repositoryOwner.ID)
	assert.Equal(t, "/-/codespaces?owner=user2&partial=true", ctx.Data["CodespaceListStateURL"])

	ctx, resp = contexttest.MockContext(t, "GET /-/codespaces?owner=org3&q=repo3&partial=true", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-org-list")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok = ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, organizationRepositoryID, rows[0].ID)
	repositoryOwner, ok = ctx.Data["CodespaceRepositoryOwner"].(*user_model.User)
	require.True(t, ok)
	assert.EqualValues(t, 3, repositoryOwner.ID)
	assert.Equal(t, "repo3", ctx.Data["Keyword"])
	assert.Equal(t, "/-/codespaces?owner=org3&partial=true&q=repo3", ctx.Data["CodespaceListStateURL"])
	pager, ok := ctx.Data["Page"].(*context.Pagination)
	require.True(t, ok)
	params, err := url.ParseQuery(string(pager.GetParams()))
	require.NoError(t, err)
	assert.Equal(t, url.Values{"owner": {"org3"}, "q": {"repo3"}}, params)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))

	ctx, resp = contexttest.MockContext(t, "GET /-/codespaces?owner=org17", contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-empty-owner")})
	contexttest.LoadUser(t, ctx, 2)
	List(ctx)

	require.Equal(t, http.StatusOK, resp.Code)
	rows, ok = ctx.Data["Codespaces"].([]*codespace_service.CreatorCodespaceView)
	require.True(t, ok)
	assert.Empty(t, rows)
	repositoryOwner, ok = ctx.Data["CodespaceRepositoryOwner"].(*user_model.User)
	require.True(t, ok)
	assert.EqualValues(t, 17, repositoryOwner.ID)
}

func TestDetailRendersCreatorCodespaceNoStore(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceID := int64(1003)
	insertWebViewCodespace(t, &codespace_model.Codespace{
		ID:     codespaceID,
		Status: codespace_model.StatusCreating,
	})

	for _, tc := range []struct {
		name     string
		query    string
		tab      string
		explicit bool
	}{
		{name: "default logs", tab: codespace_service.DetailModeLogs},
		{name: "explicit overview", query: "?tab=overview", tab: codespace_service.DetailModeOverview, explicit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, resp := contexttest.MockContext(t, "GET /-/codespaces/"+strconv.FormatInt(codespaceID, 10)+tc.query, contexttest.MockContextOption{Render: templates.PageRenderer(), SessionStore: session.NewMockMemStore("codespace-detail")})
			contexttest.LoadUser(t, ctx, 1)
			ctx.SetPathParam("codespace_id", strconv.FormatInt(codespaceID, 10))
			Detail(ctx)

			require.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
			view, ok := ctx.Data["Codespace"].(*codespace_service.CreatorCodespaceView)
			require.True(t, ok)
			assert.Equal(t, codespaceID, view.ID)
			assert.Equal(t, tc.tab, ctx.Data["CodespaceTab"])
			assert.Equal(t, tc.explicit, ctx.Data["CodespaceTabExplicit"])
		})
	}
}

func TestDetailOpensGatewayRecoveryModal(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertWebOpenManager(t, "https://gateway.example.com")
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("gateway_ssh_addr", "gateway_ssh_host_key_algorithm", "gateway_ssh_host_key_fingerprint_sha256").Update(&codespace_model.Manager{
		GatewaySSHAddr:                     "ssh.example.com:2222",
		GatewaySSHHostKeyAlgorithm:         "ssh-ed25519",
		GatewaySSHHostKeyFingerprintSHA256: "SHA256:view",
	})
	require.NoError(t, err)
	codespaceID := int64(1004)
	insertWebOpenCodespace(t, manager.ID, codespaceID, 94)
	manager.GatewaySSHAddr = "ssh.example.com:2222"
	metadata := webOpenRuntimeMetadata(t, 94, []*codespacev1.RuntimeEndpoint{{EndpointId: "app-3000", Label: "App"}})
	metadata.Ssh = "ssh://reported-runtime@ssh.example.com:2222"
	require.NoError(t, codespace_service.ReportRuntimeMetadata(t.Context(), manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceID:        codespaceID,
		Metadata:           metadata,
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
	assert.Contains(t, resp.Body.String(), "vscode://vscode-remote/ssh-remote&#43;reported-runtime@ssh.example.com:2222/workspaces/project")
	assert.Contains(t, resp.Body.String(), "zed://ssh/reported-runtime@ssh.example.com:2222/workspaces/project")
	assert.NotContains(t, resp.Body.String(), "#ZgotmplZ")
}

func TestDetailRejectsOtherCreator(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceID := int64(1005)
	insertWebViewCodespace(t, &codespace_model.Codespace{
		ID:     codespaceID,
		Status: codespace_model.StatusStopped,
	})

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
	err := db.Insert(t.Context(), codespace)
	require.NoError(t, err)
}
