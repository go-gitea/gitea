// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
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
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/repo1/codespaces"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/user/settings/codespaces"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/user/settings/codespaces/reset_registration_token"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user/settings/codespaces", map[string]string{
			"action": "unknown",
		}), http.StatusBadRequest)

		manager := &codespace_model.Manager{
			Name:           "integration-manager",
			RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
			TagsJSON:       `["default"]`,
			LastOnlineUnix: time.Now().Unix(),
			CreatedUnix:    time.Now().Unix(),
			MetaJSON:       "{}",
		}
		manager.GenerateManagerSecret()
		require.NoError(t, db.Insert(t.Context(), manager))

		created := user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user2/repo1/codespaces", map[string]string{
			"ref_type": "branch",
			"ref_name": "master",
		}), http.StatusSeeOther)
		location := created.Header().Get("Location")
		require.True(t, strings.HasPrefix(location, "/-/codespaces/"))
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, location), http.StatusOK)
		loginUser(t, "user4").MakeRequest(t, NewRequest(t, http.MethodGet, location), http.StatusForbidden)

		adminSession := loginUser(t, "user1")
		adminSession.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces"), http.StatusOK)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/-/admin/codespaces"), http.StatusForbidden)
		user2Session.MakeRequest(t, NewRequest(t, http.MethodGet, "/org/org3/settings/codespaces"), http.StatusOK)

		forceDeleteURL := "/-/admin/codespaces/" + strings.TrimPrefix(location, "/-/codespaces/") + "/force-delete"
		adminSession.MakeRequest(t, NewRequest(t, http.MethodPost, forceDeleteURL), http.StatusBadRequest)
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
		token := createRunningCodespaceTokenForRepo(t, repo)

		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/user").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/version").AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/signing-key.pub").AddTokenAuth(token), http.StatusNotFound)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/branches", repo.OwnerName, repo.Name).AddTokenAuth(token), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/user5/repo4").AddTokenAuth(token), http.StatusOK)

		MakeRequest(t, NewRequestf(t, http.MethodGet, "/api/v1/repositories/%d", repo.ID).AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/repos/issues/search").AddTokenAuth(token), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, "/api/v1/users/user2/tokens").AddTokenAuth(token), http.StatusForbidden)
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
		created := user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/user2/repo1/codespaces", map[string]string{
			"ref_type": "branch",
			"ref_name": "master",
		}), http.StatusSeeOther)
		codespaceUUID := strings.TrimPrefix(created.Header().Get("Location"), "/-/codespaces/")
		require.NoError(t, codespace_model.ValidateUUID(codespaceUUID))

		row := loadIntegrationCodespace(t, codespaceUUID)
		require.Equal(t, codespace_model.StatusCreating, row.Status)
		require.Equal(t, codespace_model.OperationCreate, row.OperationType)
		require.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
		require.Equal(t, codespace_model.OperationTriggerUser, row.OperationTrigger)
		require.EqualValues(t, 1, row.OperationRVersion)

		fetched, err := client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:   1,
			CapacityAvailable: 1,
			AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
				codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE,
			},
			MaxOperations: 1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetCreate())

		_, err = client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, 1, codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "gitea_token_required", integrationFailureCategory(t, err))

		tokenResponse, err := client.RequestGiteaToken(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestGiteaTokenRequest{
			ProtocolVersion: 1,
			CodespaceUuid:   codespaceUUID,
		}))
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(tokenResponse.Msg.GetToken(), "gcs_"))

		_, err = client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, 1, codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Equal(t, "metadata_required", integrationFailureCategory(t, err))

		_, err = client.ReportRuntimeMetadata(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
			ProtocolVersion:    1,
			CodespaceUuid:      codespaceUUID,
			MetadataGeneration: 1,
			MetadataJson:       integrationRuntimeMetadataJSON(t, 1),
		}))
		require.NoError(t, err)

		finalCreate, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, 1, codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.NotNil(t, finalCreate.Msg.GetFinalAccepted())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)
		assert.Empty(t, row.OperationType)
		assertIntegrationExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

		idleStop, err := client.RequestIdleStop(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
			ProtocolVersion:               1,
			CodespaceUuid:                 codespaceUUID,
			ObservedAutoStopEnabled:       true,
			ObservedIdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
			ObservedInteractionGeneration: row.InteractionGeneration,
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
			MaxOperations:            1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetStop())
		stopVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		finalStop, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, stopVersion, codespacev1.OperationType_OPERATION_TYPE_STOP, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.NotNil(t, finalStop.Msg.GetFinalAccepted())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusStopped, row.Status)
		assert.Empty(t, row.OperationType)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/resume"), http.StatusSeeOther)
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.OperationResume, row.OperationType)
		assert.EqualValues(t, 2, row.InteractionGeneration)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:   1,
			CapacityAvailable: 1,
			AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
				codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME,
			},
			MaxOperations: 1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetResume())
		resumeVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		_, err = client.RequestGiteaToken(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestGiteaTokenRequest{
			ProtocolVersion: 1,
			CodespaceUuid:   codespaceUUID,
		}))
		require.NoError(t, err)
		_, err = client.ReportRuntimeMetadata(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
			ProtocolVersion:    1,
			CodespaceUuid:      codespaceUUID,
			MetadataGeneration: 2,
			MetadataJson:       integrationRuntimeMetadataJSON(t, resumeVersion),
		}))
		require.NoError(t, err)
		finalResume, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, resumeVersion, codespacev1.OperationType_OPERATION_TYPE_RESUME, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.NotNil(t, finalResume.Msg.GetFinalAccepted())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusRunning, row.Status)

		idleStop, err = client.RequestIdleStop(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestIdleStopRequest{
			ProtocolVersion:               1,
			CodespaceUuid:                 codespaceUUID,
			ObservedAutoStopEnabled:       true,
			ObservedIdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
			ObservedInteractionGeneration: row.InteractionGeneration,
		}))
		require.NoError(t, err)
		require.NotNil(t, idleStop.Msg.GetPending())
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			CleanupCapacityAvailable: 1,
			MaxOperations:            1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetStop())
		idleStopVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		finalIdleStop, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, idleStopVersion, codespacev1.OperationType_OPERATION_TYPE_STOP, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.NotNil(t, finalIdleStop.Msg.GetFinalAccepted())
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.StatusStopped, row.Status)
		assert.Empty(t, row.OperationType)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)

		user2Session.MakeRequest(t, NewRequest(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/resume"), http.StatusSeeOther)
		row = loadIntegrationCodespace(t, codespaceUUID)
		assert.Equal(t, codespace_model.OperationResume, row.OperationType)
		assert.EqualValues(t, 3, row.InteractionGeneration)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:   1,
			CapacityAvailable: 1,
			AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
				codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME,
			},
			MaxOperations: 1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		resumeVersion = fetched.Msg.GetOperations()[0].GetOperationRversion()
		_, err = client.RequestGiteaToken(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.RequestGiteaTokenRequest{
			ProtocolVersion: 1,
			CodespaceUuid:   codespaceUUID,
		}))
		require.NoError(t, err)
		_, err = client.ReportRuntimeMetadata(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.ReportRuntimeMetadataRequest{
			ProtocolVersion:    1,
			CodespaceUuid:      codespaceUUID,
			MetadataGeneration: 3,
			MetadataJson:       integrationRuntimeMetadataJSON(t, resumeVersion),
		}))
		require.NoError(t, err)
		finalResume, err = client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, resumeVersion, codespacev1.OperationType_OPERATION_TYPE_RESUME, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.NotNil(t, finalResume.Msg.GetFinalAccepted())
		assert.Equal(t, codespace_model.StatusRunning, loadIntegrationCodespace(t, codespaceUUID).Status)

		user2Session.MakeRequest(t, NewRequestWithValues(t, http.MethodPost, "/-/codespaces/"+codespaceUUID+"/delete", map[string]string{
			"return_to": "/-/codespaces",
		}), http.StatusSeeOther)
		assertIntegrationNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
		fetched, err = client.FetchOperations(t.Context(), codespaceManagerRequest(manager.ID, secret, &codespacev1.FetchOperationsRequest{
			ProtocolVersion:          1,
			CleanupCapacityAvailable: 1,
			MaxOperations:            1,
		}))
		require.NoError(t, err)
		require.Len(t, fetched.Msg.GetOperations(), 1)
		assert.NotNil(t, fetched.Msg.GetOperations()[0].GetDelete())
		deleteVersion := fetched.Msg.GetOperations()[0].GetOperationRversion()
		finalDelete, err := client.FinalizeOperation(t.Context(), codespaceManagerRequest(manager.ID, secret, createFinalRequest(codespaceUUID, deleteVersion, codespacev1.OperationType_OPERATION_TYPE_DELETE, codespacev1.FinalStatus_FINAL_STATUS_DONE)))
		require.NoError(t, err)
		assert.NotNil(t, finalDelete.Msg.GetFinalAccepted())
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
		assert.Nil(t, inventory.Msg.GetResults()[0].GetAction())
		assert.EqualValues(t, 21, inventory.Msg.GetResults()[0].GetRuntimeSettings().GetInteractionGeneration())
		assert.EqualValues(t, 12, inventory.Msg.GetResults()[1].GetRefetchOperation().GetCurrentOperationRversion())
		assert.EqualValues(t, 13, inventory.Msg.GetResults()[2].GetClearOperationContext().GetCurrentOperationRversion())
		assert.EqualValues(t, 14, inventory.Msg.GetResults()[3].GetReportRuntimeTransition().GetCurrentOperationRversion())
		assert.EqualValues(t, 15, inventory.Msg.GetResults()[4].GetReportRuntimeTransition().GetCurrentOperationRversion())
		assert.EqualValues(t, 16, inventory.Msg.GetResults()[5].GetStopLocalRuntime().GetCurrentOperationRversion())
		assert.NotNil(t, inventory.Msg.GetResults()[6].GetCleanupLocalRuntime())
		assert.Nil(t, inventory.Msg.GetResults()[6].GetRuntimeSettings())
		assert.NotNil(t, inventory.Msg.GetResults()[7].GetCleanupLocalRuntime())
		assert.Nil(t, inventory.Msg.GetResults()[8].GetAction())
		assert.Nil(t, inventory.Msg.GetResults()[8].GetRuntimeSettings())
		assert.Nil(t, inventory.Msg.GetResults()[9].GetAction())
		assert.NotNil(t, inventory.Msg.GetResults()[9].GetRuntimeSettings())
		assert.Nil(t, inventory.Msg.GetResults()[10].GetAction())
		assert.NotNil(t, inventory.Msg.GetResults()[10].GetRuntimeSettings())
		assert.NotNil(t, inventory.Msg.GetResults()[11].GetCleanupLocalRuntime())
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
		assert.Equal(t, "state_history_conflict", integrationFailureCategory(t, err))
		assert.EqualValues(t, 1, loadIntegrationManager(t, manager.ID).InventoryGeneration)
		assert.Equal(t, codespace_model.StatusRunning, loadIntegrationCodespace(t, runningUUID).Status)
	})
}

func createRunningCodespaceTokenForRepo(t *testing.T, repo *repo_model.Repository) string {
	t.Helper()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	codespaceUUID := codespace_model.NewUUID()
	manager := &codespace_model.Manager{
		Name:           "codespace-token-manager-" + codespaceUUID[:8],
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       `["default"]`,
		LastOnlineUnix: time.Now().Unix(),
		CreatedUnix:    time.Now().Unix(),
		MetaJSON:       "{}",
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))

	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:              codespaceUUID,
		UserID:            user.ID,
		RepoID:            repo.ID,
		RefType:           "branch",
		RefName:           repo.DefaultBranch,
		RepoTag:           "default",
		GitProtocol:       codespace_model.GitProtocolHTTP,
		CommitSHA:         "0123456789abcdef0123456789abcdef01234567",
		ManagerID:         manager.ID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 1,
		AutoStopMode:      codespace_model.AutoStopModeDefault,
		CreatedUnix:       time.Now().Unix(),
		UpdatedUnix:       time.Now().Unix(),
		LogFilename:       codespaceUUID + ".log",
	}))

	result, err := codespace_service.RequestGiteaToken(t.Context(), manager, codespace_service.RequestGiteaTokenOptions{
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	return result.Token
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
	if codespace.RepoTag == "" {
		codespace.RepoTag = "default"
	}
	if codespace.GitProtocol == "" {
		codespace.GitProtocol = codespace_model.GitProtocolHTTP
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
	if codespace.LogFilename == "" {
		codespace.LogFilename = codespace.UUID + ".log"
	}
	require.NoError(t, db.Insert(t.Context(), codespace))
}

func createIntegrationManager(t *testing.T) (*codespace_model.Manager, string) {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "integration-state-manager",
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       `["default"]`,
		LastOnlineUnix: time.Now().Unix(),
		CreatedUnix:    time.Now().Unix(),
		MetaJSON:       "{}",
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
		Final: &codespacev1.FinalResult{
			Status:        finalStatus,
			OperationType: operationType,
		},
	}
}

func integrationRuntimeMetadataJSON(t *testing.T, operationRVersion int64) string {
	t.Helper()
	payload := map[string]any{
		"endpoints": []map[string]any{
			{
				"endpoint_id": "workspace",
				"label":       "Workspace",
				"public":      false,
			},
		},
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

func integrationFailureCategory(t *testing.T, err error) string {
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
