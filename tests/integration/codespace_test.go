// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodespaceRoutes(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		MakeRequest(t, NewRequest(t, http.MethodGet, "/-/codespaces"), http.StatusSeeOther)

		user2Session := loginUser(t, "user2")
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user/settings/codespaces/managers"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user/settings/codespaces/permissions"), http.StatusOK)
		now := time.Now().Unix()
		authorization := &codespace_model.PermissionAuthorization{
			UserID: 2, SourceRepoID: 1, RequestHash: "settings-route",
			CreatedUnix: now, UpdatedUnix: now,
		}
		require.NoError(t, db.Insert(t.Context(), authorization))
		rule := &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), rule))
		permissionForm := map[string]string{
			"action": "reduce", "authorization_id": strconv.FormatInt(authorization.ID, 10),
			"rule_id": strconv.FormatInt(rule.ID, 10), "mode": "read",
		}
		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user/settings/codespaces/permissions", permissionForm), http.StatusSeeOther)
		rule = unittest.AssertExistsAndLoadBean(t, &codespace_model.PermissionRepository{ID: rule.ID})
		assert.Equal(t, perm.AccessModeRead, rule.GrantedMode)
		loginUser(t, "user4").MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user/settings/codespaces/permissions", permissionForm), http.StatusNotFound)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/user/settings/codespaces/managers/reset_registration_token"), http.StatusOK)

		manager := &codespace_model.Manager{
			Name:           "integration-manager",
			UserID:         2,
			RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
			TagsJSON:       `[{"tag":"default"}]`,
			LastOnlineUnix: time.Now().Unix(),
			CreatedUnix:    time.Now().Unix(),
		}
		manager.GenerateManagerSecret()
		require.NoError(t, db.Insert(t.Context(), manager))
		user2Session.MakeRequest(t, NewRequestf(t, http.MethodGet, "/user/settings/codespaces/managers/%d", manager.ID), http.StatusOK)
		loginUser(t, "user4").MakeRequest(t, NewRequestf(t, http.MethodGet, "/user/settings/codespaces/managers/%d", manager.ID), http.StatusNotFound)

		created := createCodespaceFromRepository(t, user2Session, "/user2/repo1/codespaces", "branch", "master")
		location := created.Header().Get("Location")
		require.True(t, strings.HasPrefix(location, "/-/codespaces/"))
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, location), http.StatusOK)
		loginUser(t, "user4").MakeRequest(t, NewRequest(t, http.MethodGet, location), http.StatusNotFound)

		adminSession := loginUser(t, "user1")
		adminSession.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces/managers"), http.StatusOK)
		adminSession.MakeRequest(t, NewRequestf(t, http.MethodGet, "/-/admin/codespaces/managers/%d", manager.ID), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces/managers"), http.StatusForbidden)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/org/org3/settings/codespaces"), http.StatusNotFound)

		forceDeleteURL := "/-/admin/codespaces/managers/unassigned/" + strings.TrimPrefix(location, "/-/codespaces/") + "/force-delete"
		adminSession.MakeRequest(t, NewRequest(t, http.MethodPost, forceDeleteURL), http.StatusSeeOther)
		adminSession.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, forceDeleteURL, map[string]string{
			"confirm": "force-delete",
		}), http.StatusSeeOther)

		client := codespacev1connect.NewManagerServiceClient(
			http.DefaultClient,
			strings.TrimRight(giteaURL.String(), "/")+"/api/codespace",
		)
		_, err := client.RegisterManager(t.Context(), connect.NewRequest(&codespacev1.RegisterManagerRequest{
			ProtocolVersion:   0,
			RegistrationToken: "missing",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})
}

func TestCodespaceTokenAPIRoutePolicy(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		defer test.MockVariableValue(&setting.Codespace.Enabled, true)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		token, codespaceUUID := createRunningCodespaceTokenForRepo(t, repo)

		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/user").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/version").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/signing-key.pub").AddTokenAuth(token), http.StatusNotFound)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/branches", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/forks", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/runs", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/actions/workflows/missing/dispatches", repo.OwnerName, repo.Name), map[string]string{}).AddTokenAuth(token), http.StatusUnprocessableEntity)
		MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/markdown", repo.OwnerName, repo.Name), &api.MarkdownOption{Text: "codespace"}).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user5/repo4").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1/info/refs?service=git-upload-pack").AddBasicAuth("codespace", token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1.git/info/lfs/locks").AddBasicAuth("codespace", token).SetHeader("Accept", lfs.MediaType), http.StatusOK)

		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repositories/%d", repo.ID).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/issues/search").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/users/user2/tokens").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestf(t, http.MethodPost, "/api/v1/repos/%s/%s/forks", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/secrets", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusForbidden)
		personalToken := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/secrets", repo.OwnerName, repo.Name).AddTokenAuth(personalToken), http.StatusOK)

		now := time.Now().Unix()
		pullUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypePullRequests})
		pullUnit.ID = 0
		pullUnit.RepoID = 2
		require.NoError(t, db.Insert(t.Context(), pullUnit))
		authorization := &codespace_model.PermissionAuthorization{
			UserID: 2, SourceRepoID: repo.ID, RequestHash: "integration-repository-grant",
			CreatedUnix: now, UpdatedUnix: now,
		}
		require.NoError(t, db.Insert(t.Context(), authorization))
		rule := &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		}
		require.NoError(t, db.Insert(t.Context(), rule))
		for _, unitType := range []unit.Type{unit.TypeIssues, unit.TypePullRequests, unit.TypeWiki, unit.TypeReleases, unit.TypeActions} {
			require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
				AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unitType,
				RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
			}))
		}
		require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 4, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeWrite, GrantedMode: perm.AccessModeWrite,
		}))
		updated, err := db.GetEngine(t.Context()).ID(codespaceUUID).Cols("permission_authorization_id").Update(&codespace_model.Codespace{PermissionAuthorizationID: authorization.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, updated)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusOK)
		for _, tt := range []struct {
			path   string
			status int
		}{
			{path: "/api/v1/repos/user2/repo2/issues", status: http.StatusOK},
			{path: "/api/v1/repos/user2/repo2/pulls", status: http.StatusOK},
			{path: "/api/v1/repos/user2/repo2/wiki/pages", status: http.StatusNotFound}, // Permission passes before the absent wiki repository is reported.
			{path: "/api/v1/repos/user2/repo2/releases", status: http.StatusOK},
			{path: "/api/v1/repos/user2/repo2/actions/runs", status: http.StatusOK},
		} {
			MakeRequest(t, NewRequest(t, http.MethodGet, tt.path).AddTokenAuth(token), tt.status)
		}
		for _, path := range []string{
			"/api/v1/repos/user2/repo2/issues",
			"/api/v1/repos/user2/repo2/pulls",
			"/api/v1/repos/user2/repo2/wiki/new",
			"/api/v1/repos/user2/repo2/releases",
			"/api/v1/repos/user2/repo2/actions/workflows/missing/dispatches",
		} {
			MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, path, map[string]string{}).AddTokenAuth(token), http.StatusForbidden)
		}
		snapshot, err := codespace_service.ResolveGiteaToken(t.Context(), token)
		require.NoError(t, err)
		require.True(t, snapshot.CodespaceTokenAllowsRepository(4, unit.TypeCode, perm.AccessModeWrite))
		MakeRequest(t, NewRequestWithJSON(t, http.MethodPost, "/api/v1/repos/user5/repo4/branches", map[string]string{}).AddTokenAuth(token), http.StatusForbidden)

		require.NoError(t, codespace_service.ReducePermissionRepository(t.Context(), 2, authorization.ID, rule.ID, perm.AccessModeNone))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)

		authorization = &codespace_model.PermissionAuthorization{
			UserID: 2, SourceRepoID: repo.ID, RequestHash: "integration-repository-revoke",
			CreatedUnix: now, UpdatedUnix: now,
		}
		require.NoError(t, db.Insert(t.Context(), authorization))
		require.NoError(t, db.Insert(t.Context(), &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID, TargetRepoID: 2, UnitType: unit.TypeCode,
			RequestedMode: perm.AccessModeRead, GrantedMode: perm.AccessModeRead,
		}))
		updated, err = db.GetEngine(t.Context()).ID(codespaceUUID).Cols("permission_authorization_id").Update(&codespace_model.Codespace{PermissionAuthorizationID: authorization.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, updated)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusOK)
		require.NoError(t, codespace_service.RevokePermissionAuthorization(t.Context(), 2, authorization.ID))
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)

		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/actions/artifacts/999999/zip/raw", repo.OwnerName, repo.Name).AddTokenAuth("gcs_invalid"), http.StatusNotFound)
	})
}

func TestCodespaceLifecycleStateMachineIntegration(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		manager, secret := createIntegrationManager(t)
		client := codespacev1connect.NewManagerServiceClient(
			http.DefaultClient,
			strings.TrimRight(giteaURL.String(), "/")+"/api/codespace",
		)

		user2Session := loginUser(t, "user2")
		created := createCodespaceFromRepository(t, user2Session, "/user2/repo1/codespaces", "branch", "master")
		codespaceUUID := strings.TrimPrefix(created.Header().Get("Location"), "/-/codespaces/")
		require.NoError(t, codespace_model.ValidateUUID(codespaceUUID))

		row := loadIntegrationCodespace(t, codespaceUUID)
		require.Equal(t, codespace_model.StatusCreating, row.Status)
		require.Equal(t, codespace_model.OperationCreate, row.OperationType)
		require.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
		require.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
		require.EqualValues(t, 1, row.OperationRVersion)

		fetched, err := client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			StartupCapacityAvailable: 1,
			AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
				codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE,
			},
			AcceptedCreateTags: []string{"default"},
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetCreate())

		_, err = client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, 1, codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "gitea_token_required", integrationFailureDetailCategory(t, err))

		tokenResponse, err := client.RequestRuntimeAccess(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestRuntimeAccessRequest{
			ProtocolVersion:   1,
			CodespaceUuid:     codespaceUUID,
			OperationRversion: 1,
			GitSshPublicKey:   integrationGitSSHPublicKey(t),
		}))
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(tokenResponse.Msg.GetToken(), "gcs_"))

		_, err = client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, 1, codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "metadata_required", integrationFailureDetailCategory(t, err))

		_, err = client.ReportRuntimeMetadata(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
			ProtocolVersion:    1,
			CodespaceUuid:      codespaceUUID,
			MetadataGeneration: 1,
			Metadata:           integrationRuntimeMetadata(1),
		}))
		require.NoError(t, err)

		finalCreate, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, 1, codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.False(t, finalCreate.Msg.GetResourceAbsent())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)
		assert.Empty(t, row.OperationType)
		assertIntegrationExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

		autoStopResponse := user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/auto-stop", map[string]string{
			"mode":          "custom",
			"timeout_value": "30",
			"timeout_unit":  "minutes",
			"return_to":     "detail",
		}), http.StatusSeeOther)
		assert.Equal(t, "/-/codespaces/"+codespaceUUID, autoStopResponse.Header().Get("Location"))
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.AutoStopModeCustom, row.AutoStopMode)
		assert.EqualValues(t, 30*60, row.AutoStopTimeoutSeconds)

		idleStop, err := client.RequestIdleStop(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
			ProtocolVersion: 1,
			CodespaceUuid:   codespaceUUID,
			ObservedSettings: &codespacev1.EffectiveCodespaceRuntimeSettings{
				AutoStopEnabled:       true,
				IdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
				InteractionGeneration: row.InteractionGeneration,
			},
		}))
		require.NoError(t, err)
		require.NotNil(t, idleStop.Msg.GetPending())
		assert.EqualValues(t, 2, idleStop.Msg.GetPending().GetOperationRversion())

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/continue"), http.StatusSeeOther)
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)
		assert.Empty(t, row.OperationType)
		assert.EqualValues(t, 1, row.InteractionGeneration)

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/stop"), http.StatusSeeOther)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			CleanupCapacityAvailable: 1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetStop())
		stopVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		finalStop, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, stopVersion, codespacev1.OperationType_OPERATION_TYPE_STOP, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.False(t, finalStop.Msg.GetResourceAbsent())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusStopped, row.Status)
		assert.Empty(t, row.OperationType)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/resume"), http.StatusSeeOther)
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.OperationResume, row.OperationType)
		assert.EqualValues(t, 2, row.InteractionGeneration)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			StartupCapacityAvailable: 1,
			AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
				codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME,
			},
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetResume())
		resumeVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		_, err = client.RequestRuntimeAccess(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestRuntimeAccessRequest{
			ProtocolVersion:   1,
			CodespaceUuid:     codespaceUUID,
			OperationRversion: resumeVersion,
			GitSshPublicKey:   integrationGitSSHPublicKey(t),
		}))
		require.NoError(t, err)
		_, err = client.ReportRuntimeMetadata(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
			ProtocolVersion:    1,
			CodespaceUuid:      codespaceUUID,
			MetadataGeneration: 2,
			Metadata:           integrationRuntimeMetadata(resumeVersion),
		}))
		require.NoError(t, err)
		finalResume, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, resumeVersion, codespacev1.OperationType_OPERATION_TYPE_RESUME, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.False(t, finalResume.Msg.GetResourceAbsent())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)

		idleStop, err = client.RequestIdleStop(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
			ProtocolVersion: 1,
			CodespaceUuid:   codespaceUUID,
			ObservedSettings: &codespacev1.EffectiveCodespaceRuntimeSettings{
				AutoStopEnabled:       true,
				IdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
				InteractionGeneration: row.InteractionGeneration,
			},
		}))
		require.NoError(t, err)
		require.NotNil(t, idleStop.Msg.GetPending())
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			CleanupCapacityAvailable: 1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetStop())
		idleStopVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		finalIdleStop, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, idleStopVersion, codespacev1.OperationType_OPERATION_TYPE_STOP, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.False(t, finalIdleStop.Msg.GetResourceAbsent())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusStopped, row.Status)
		assert.Empty(t, row.OperationType)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/resume"), http.StatusSeeOther)
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.OperationResume, row.OperationType)
		assert.EqualValues(t, 3, row.InteractionGeneration)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			StartupCapacityAvailable: 1,
			AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
				codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME,
			},
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		resumeVersion = fetched.Msg.GetOperations()[0].GetOperationRversion()
		_, err = client.RequestRuntimeAccess(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestRuntimeAccessRequest{
			ProtocolVersion:   1,
			CodespaceUuid:     codespaceUUID,
			OperationRversion: resumeVersion,
			GitSshPublicKey:   integrationGitSSHPublicKey(t),
		}))
		require.NoError(t, err)
		_, err = client.ReportRuntimeMetadata(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
			ProtocolVersion:    1,
			CodespaceUuid:      codespaceUUID,
			MetadataGeneration: 3,
			Metadata:           integrationRuntimeMetadata(resumeVersion),
		}))
		require.NoError(t, err)
		finalResume, err = client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, resumeVersion, codespacev1.OperationType_OPERATION_TYPE_RESUME, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.False(t, finalResume.Msg.GetResourceAbsent())
		assert.Equal(t, codespace_model.StatusRunning, loadIntegrationCodespace(t, codespaceUUID).Status)

		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/delete", map[string]string{
			"return_to": "/-/codespaces",
		}), http.StatusSeeOther)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			CleanupCapacityAvailable: 1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetDelete())
		deleteVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		finalDelete, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, deleteVersion, codespacev1.OperationType_OPERATION_TYPE_DELETE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.False(t, finalDelete.Msg.GetResourceAbsent())
		assertIntegrationNotExists(t, new(codespace_model.Codespace), "uuid = ?", codespaceUUID)
	})
}

func TestCodespaceInventoryStateMachineIntegration(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		manager, secret := createIntegrationManager(t)
		otherManager, _ := createIntegrationManager(t)
		client := codespacev1connect.NewManagerServiceClient(
			http.DefaultClient,
			strings.TrimRight(giteaURL.String(), "/")+"/api/codespace",
		)
		now := time.Now().Unix()

		runningUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:                  runningUUID,
			Status:                codespace_model.StatusRunning,
			OperationRVersion:     11,
			InteractionGeneration: 21,
		})
		refetchUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:                  refetchUUID,
			Status:                codespace_model.StatusRunning,
			OperationRVersion:     12,
			OperationType:         codespace_model.OperationStop,
			OperationStatus:       codespace_model.OperationStatusQueued,
			OperationTrigger:      codespace_model.OperationTriggerUser,
			OperationCreatedUnix:  now,
			OperationDeadlineUnix: now + int64(time.Hour/time.Second),
		})
		clearUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:              clearUUID,
			Status:            codespace_model.StatusRunning,
			OperationRVersion: 13,
		})
		reportStoppedUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:              reportStoppedUUID,
			Status:            codespace_model.StatusRunning,
			OperationRVersion: 14,
		})
		reportFailedUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:              reportFailedUUID,
			Status:            codespace_model.StatusStopped,
			OperationRVersion: 15,
		})
		stopUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:              stopUUID,
			Status:            codespace_model.StatusStopped,
			OperationRVersion: 16,
		})
		failedCleanupUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:              failedCleanupUUID,
			Status:            codespace_model.StatusFailed,
			OperationRVersion: 17,
		})
		otherBindingUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, otherManager.ID, &codespace_model.Codespace{
			UUID:              otherBindingUUID,
			Status:            codespace_model.StatusRunning,
			OperationRVersion: 18,
		})
		unboundCreatingUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, 0, &codespace_model.Codespace{
			UUID:                 unboundCreatingUUID,
			Status:               codespace_model.StatusCreating,
			OperationRVersion:    19,
			OperationType:        codespace_model.OperationCreate,
			OperationStatus:      codespace_model.OperationStatusQueued,
			OperationTrigger:     codespace_model.OperationTriggerUser,
			OperationCreatedUnix: now,
		})
		activeNoContextUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:                 activeNoContextUUID,
			Status:               codespace_model.StatusRunning,
			OperationRVersion:    20,
			OperationType:        codespace_model.OperationStop,
			OperationStatus:      codespace_model.OperationStatusQueued,
			OperationTrigger:     codespace_model.OperationTriggerUser,
			OperationCreatedUnix: now,
		})
		activeSameFailedUUID := codespace_model.NewUUID()
		insertIntegrationCodespace(t, manager.ID, &codespace_model.Codespace{
			UUID:                 activeSameFailedUUID,
			Status:               codespace_model.StatusRunning,
			OperationRVersion:    21,
			OperationType:        codespace_model.OperationStop,
			OperationStatus:      codespace_model.OperationStatusRunning,
			OperationTrigger:     codespace_model.OperationTriggerUser,
			OperationCreatedUnix: now,
		})
		absentUUID := codespace_model.NewUUID()

		inventory, err := client.ReportInstances(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportInstancesRequest{
			ProtocolVersion:     1,
			InventoryGeneration: 1,
			Instances: []*codespacev1.RuntimeInstanceRef{
				{CodespaceUuid: runningUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
				{CodespaceUuid: refetchUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING, ObservedOperationRversion: 11},
				{CodespaceUuid: clearUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING, ObservedOperationRversion: 13},
				{CodespaceUuid: reportStoppedUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_STOPPED},
				{CodespaceUuid: reportFailedUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_FAILED},
				{CodespaceUuid: stopUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
				{CodespaceUuid: failedCleanupUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_FAILED},
				{CodespaceUuid: otherBindingUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
				{CodespaceUuid: unboundCreatingUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_CREATING},
				{CodespaceUuid: activeNoContextUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_RUNNING},
				{CodespaceUuid: activeSameFailedUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_FAILED, ObservedOperationRversion: 21},
				{CodespaceUuid: absentUUID, RuntimeState: codespacev1.RuntimeState_RUNTIME_STATE_FAILED},
			},
		}))
		require.NoError(t, err)
		require.Len(t, inventory.Msg.GetResults(), 12)
		assert.Equal(t, runningUUID, inventory.Msg.GetResults()[0].GetCodespaceUuid())
		assert.NotNil(t, inventory.Msg.GetResults()[0].GetRuntimeSettings())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, inventory.Msg.GetResults()[0].GetAction())
		assert.EqualValues(t, 21, inventory.Msg.GetResults()[0].GetRuntimeSettings().GetInteractionGeneration())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REFETCH_OPERATION, inventory.Msg.GetResults()[1].GetAction())
		assert.EqualValues(t, 12, inventory.Msg.GetResults()[1].GetCurrentOperationRversion())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEAR_OPERATION_CONTEXT, inventory.Msg.GetResults()[2].GetAction())
		assert.EqualValues(t, 13, inventory.Msg.GetResults()[2].GetCurrentOperationRversion())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REPORT_RUNTIME_TRANSITION, inventory.Msg.GetResults()[3].GetAction())
		assert.EqualValues(t, 14, inventory.Msg.GetResults()[3].GetCurrentOperationRversion())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REPORT_RUNTIME_TRANSITION, inventory.Msg.GetResults()[4].GetAction())
		assert.EqualValues(t, 15, inventory.Msg.GetResults()[4].GetCurrentOperationRversion())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_STOP_LOCAL_RUNTIME, inventory.Msg.GetResults()[5].GetAction())
		assert.EqualValues(t, 16, inventory.Msg.GetResults()[5].GetCurrentOperationRversion())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, inventory.Msg.GetResults()[6].GetAction())
		assert.Nil(t, inventory.Msg.GetResults()[6].GetRuntimeSettings())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, inventory.Msg.GetResults()[7].GetAction())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, inventory.Msg.GetResults()[8].GetAction())
		assert.Nil(t, inventory.Msg.GetResults()[8].GetRuntimeSettings())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, inventory.Msg.GetResults()[9].GetAction())
		assert.NotNil(t, inventory.Msg.GetResults()[9].GetRuntimeSettings())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_UNSPECIFIED, inventory.Msg.GetResults()[10].GetAction())
		assert.NotNil(t, inventory.Msg.GetResults()[10].GetRuntimeSettings())
		assert.Equal(t, codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME, inventory.Msg.GetResults()[11].GetAction())
		assert.EqualValues(t, 1, loadIntegrationManager(t, manager.ID).InventoryGeneration)

		_, err = client.ReportInstances(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportInstancesRequest{
			ProtocolVersion:     1,
			InventoryGeneration: 2,
			Instances: []*codespacev1.RuntimeInstanceRef{{
				CodespaceUuid:             runningUUID,
				RuntimeState:              codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
				ObservedOperationRversion: 12,
			}},
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "state_history_conflict", integrationFailureDetailCategory(t, err))
		assert.EqualValues(t, 1, loadIntegrationManager(t, manager.ID).InventoryGeneration)
		assert.Equal(t, codespace_model.StatusRunning, loadIntegrationCodespace(t, runningUUID).Status)
	})
}

func createCodespaceFromRepository(t *testing.T, session *TestSession, path, refType, refName string) *httptest.ResponseRecorder {
	t.Helper()
	values := map[string]string{"ref_type": refType, "ref_name": refName}
	query := url.Values{}
	query.Set("ref_type", refType)
	query.Set("ref_name", refName)
	confirmation := session.MakeRequest(t, NewRequest(t, http.MethodGet, path+"/new?"+query.Encode()), http.StatusOK)
	doc := NewHTMLParser(t, confirmation.Body).doc
	requestHash, ok := doc.Find(`input[name="request_hash"]`).Attr("value")
	require.True(t, ok)
	require.NotEmpty(t, requestHash)
	environmentTag, ok := doc.Find(`.codespace-create-environment-option`).First().Attr("data-value")
	require.True(t, ok)
	require.NotEmpty(t, environmentTag)
	values["request_hash"] = requestHash
	values["environment_tag"] = environmentTag
	return session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, path, values), http.StatusSeeOther)
}

func createRunningCodespaceTokenForRepo(t *testing.T, repo *repo_model.Repository) (string, string) {
	t.Helper()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	codespaceUUID := codespace_model.NewUUID()
	manager := &codespace_model.Manager{
		Name:           "codespace-token-manager-" + codespaceUUID[:8],
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       `[{"tag":"default"}]`,
		LastOnlineUnix: time.Now().Unix(),
		CreatedUnix:    time.Now().Unix(),
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))

	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:              codespaceUUID,
		UserID:            user.ID,
		RepoID:            repo.ID,
		RefType:           "branch",
		RefName:           repo.DefaultBranch,
		EnvironmentTag:    "default",
		CommitSHA:         "0123456789abcdef0123456789abcdef01234567",
		ManagerID:         manager.ID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 1,
		AutoStopMode:      codespace_model.AutoStopModeDefault,
		CreatedUnix:       time.Now().Unix(),
		UpdatedUnix:       time.Now().Unix(),
	}))

	result, err := codespace_service.RequestRuntimeAccess(t.Context(), manager, codespace_service.RequestRuntimeAccessOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 1,
		GitSSHPublicKey:   integrationGitSSHPublicKey(t),
	})
	require.NoError(t, err)
	return result.Token, codespaceUUID
}

func integrationGitSSHPublicKey(t *testing.T) []byte {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString("AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf")
	require.NoError(t, err)
	return key
}

func insertIntegrationCodespace(t *testing.T, managerID int64, codespace *codespace_model.Codespace) {
	t.Helper()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	now := time.Now().Unix()
	codespace.UserID = user.ID
	codespace.RepoID = repo.ID
	if codespace.RefType == "" {
		codespace.RefType = "branch"
	}
	if codespace.RefName == "" {
		codespace.RefName = repo.DefaultBranch
	}
	if codespace.EnvironmentTag == "" {
		codespace.EnvironmentTag = "default"
	}
	if codespace.CommitSHA == "" {
		codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	}
	codespace.ManagerID = managerID
	if codespace.AutoStopMode == "" {
		codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	}
	if codespace.CreatedUnix == 0 {
		codespace.CreatedUnix = now
	}
	if codespace.UpdatedUnix == 0 {
		codespace.UpdatedUnix = now
	}
	require.NoError(t, db.Insert(t.Context(), codespace))
}

func createIntegrationManager(t *testing.T) (*codespace_model.Manager, string) {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "integration-state-manager",
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       `[{"tag":"default"}]`,
		LastOnlineUnix: time.Now().Unix(),
		CreatedUnix:    time.Now().Unix(),
	}
	secret := manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager, secret
}

func loadIntegrationManager(t *testing.T, managerID int64) *codespace_model.Manager {
	t.Helper()
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(t.Context()).ID(managerID).Get(manager)
	require.NoError(t, err)
	require.True(t, has)
	return manager
}

func loadIntegrationCodespace(t *testing.T, codespaceUUID string) *codespace_model.Codespace {
	t.Helper()
	row := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).ID(codespaceUUID).Get(row)
	require.NoError(t, err)
	require.True(t, has)
	return row
}

func codespaceManagerRequest[T any](managerID int64, managerSecret string, message *T) *connect.Request[T] {
	request := connect.NewRequest(message)
	request.Header().Set("x-codespace-manager-id", strconv.FormatInt(managerID, 10))
	request.Header().Set("x-codespace-manager-secret", managerSecret)
	return request
}

func createFinalRequest(codespaceUUID string, operationRVersion int64, operationType codespacev1.OperationType, finalStatus codespacev1.FinalStatus) *codespacev1.FinalizeOperationRequest {
	return &codespacev1.FinalizeOperationRequest{
		ProtocolVersion:   1,
		CodespaceUuid:     codespaceUUID,
		OperationRversion: operationRVersion,
		Status:            finalStatus,
		OperationType:     operationType,
	}
}

func integrationRuntimeMetadata(operationRVersion int64) *codespacev1.RuntimeMetadata {
	return &codespacev1.RuntimeMetadata{
		Endpoints: []*codespacev1.RuntimeEndpoint{{EndpointId: "workspace", Label: "Workspace"}},
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

func integrationFailureDetailCategory(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		require.NoError(t, detailErr)
		if failure, ok := value.(*codespacev1.FailureDetail); ok {
			return failure.GetCategory()
		}
	}
	require.FailNow(t, "missing failure detail")
	return ""
}

func assertIntegrationExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.True(t, has)
}

func assertIntegrationNotExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.False(t, has)
}
