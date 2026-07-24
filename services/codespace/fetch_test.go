// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchOperationsClaimsCreate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureFetchGitTransport(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "10101010-1010-4010-8010-101010101010"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    31,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CapacityAvailable:      1,
		AcceptedOperationTypes: []string{AcceptedOperationCreate},
		MaxOperations:          2,
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	operation := result.Operations[0]
	assert.Equal(t, codespace_model.OperationCreate, operation.Command)
	assert.Equal(t, codespaceUUID, operation.CodespaceUUID)
	assert.EqualValues(t, 31, operation.OperationRVersion)
	assert.EqualValues(t, setting.Codespace.OperationLeaseTimeout/time.Millisecond, operation.LeaseValidForMilliseconds)
	require.NotNil(t, operation.Create)
	assert.EqualValues(t, 2, operation.Create.RepoID)
	assert.NotEmpty(t, operation.Create.RepoFullName)
	assert.NotEmpty(t, operation.Create.RepoCloneHTTPURL)
	assert.Empty(t, operation.Create.RepoCloneSSHURL)
	assert.Equal(t, codespace_model.GitProtocolHTTP, operation.Create.GitProtocol)
	assert.True(t, operation.Create.RuntimeSettings.AutoStopEnabled)
	assert.EqualValues(t, setting.Codespace.AutoStopDefaultTimeout/time.Second, operation.Create.RuntimeSettings.IdleTimeoutSeconds)

	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, manager.ID, row.ManagerID)
	assert.Equal(t, codespace_model.OperationStatusRunning, row.OperationStatus)
	assert.Positive(t, row.OperationStartedUnix)
	assert.Positive(t, row.OperationDeadlineUnix)
}

func TestFetchOperationsReturnsSSHCloneURLWhenEnabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureFetchGitTransport(t, codespace_model.GitProtocolSSH, false, false, []string{
		"localhost ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf",
	})

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "12121212-1212-4212-8212-121212121212"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    32,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
		GitProtocol:          codespace_model.GitProtocolSSH,
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CapacityAvailable:      1,
		AcceptedOperationTypes: []string{AcceptedOperationCreate},
		MaxOperations:          1,
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	require.NotNil(t, result.Operations[0].Create)
	assert.NotEmpty(t, result.Operations[0].Create.RepoCloneHTTPURL)
	assert.NotEmpty(t, result.Operations[0].Create.RepoCloneSSHURL)
	assert.Equal(t, codespace_model.GitProtocolSSH, result.Operations[0].Create.GitProtocol)
}

func configureFetchGitTransport(t *testing.T, protocol string, disableHTTPGit, disableSSH bool, knownHosts []string) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.Repository.DisableHTTPGit, disableHTTPGit))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Disabled, disableSSH))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "localhost"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 22))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, knownHosts))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitProtocol, protocol))
}

func TestFetchOperationsSkipsCreateOutsideManagerOwnerScope(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	manager.OwnerID = 3
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("owner_id").Update(manager)
	require.NoError(t, err)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "15151515-1515-4515-8515-151515151515"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    35,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CapacityAvailable:      1,
		AcceptedOperationTypes: []string{AcceptedOperationCreate},
		MaxOperations:          1,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Zero(t, row.ManagerID)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
}

func TestFetchOperationsDisabledDrainsWithoutClaimingStartup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	runningCreateUUID := "17171717-1717-4717-8717-171717171711"
	runningStopUUID := "17171717-1717-4717-8717-171717171712"
	queuedCreateUUID := "17171717-1717-4717-8717-171717171713"
	queuedStopUUID := "17171717-1717-4717-8717-171717171714"
	now := time.Now()
	originalCreateDeadline := now.Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningCreateUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     51,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  now.Add(-time.Minute).Unix(),
		OperationStartedUnix:  now.Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalCreateDeadline,
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  runningStopUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     52,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  now.Add(-time.Minute).Unix(),
		OperationStartedUnix:  now.Add(-time.Minute).Unix(),
		OperationDeadlineUnix: now.Add(time.Minute).Unix(),
	})
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 queuedCreateUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    53,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now.Unix(),
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 queuedStopUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    54,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now.Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CapacityAvailable:        1,
		AcceptedOperationTypes:   []string{AcceptedOperationCreate, AcceptedOperationResume},
		MaxOperations:            3,
		CleanupCapacityAvailable: 1,
		ObservedOperations: []ObservedOperation{
			{CodespaceUUID: runningCreateUUID, OperationRVersion: 51},
			{CodespaceUUID: runningStopUUID, OperationRVersion: 52},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 2)
	assert.Equal(t, OperationCommandAbortCreate, result.Operations[0].Command)
	assert.Zero(t, result.Operations[0].LeaseValidForMilliseconds)
	assert.Equal(t, codespace_model.OperationStop, result.Operations[1].Command)
	require.Len(t, result.RenewedLeases, 1)
	assert.Equal(t, runningStopUUID, result.RenewedLeases[0].CodespaceUUID)

	assert.Equal(t, originalCreateDeadline, loadServiceCodespace(t, runningCreateUUID).OperationDeadlineUnix)
	assert.Zero(t, loadServiceCodespace(t, queuedCreateUUID).ManagerID)
	assert.Equal(t, codespace_model.OperationStatusQueued, loadServiceCodespace(t, queuedCreateUUID).OperationStatus)
	assert.Equal(t, codespace_model.OperationStatusRunning, loadServiceCodespace(t, queuedStopUUID).OperationStatus)
}

func TestApplyQueuedTimeoutUsesQueuedStateMapping(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	now := time.Now().Unix()
	cases := []struct {
		name           string
		uuid           string
		status         string
		operationType  string
		withCredential bool
		expectedStatus string
		expectToken    bool
		expectKey      bool
	}{
		{
			name:           "create",
			uuid:           "16161616-1616-4616-8616-161616161611",
			status:         codespace_model.StatusCreating,
			operationType:  codespace_model.OperationCreate,
			withCredential: true,
			expectedStatus: codespace_model.StatusFailed,
		},
		{
			name:           "resume",
			uuid:           "16161616-1616-4616-8616-161616161612",
			status:         codespace_model.StatusStopped,
			operationType:  codespace_model.OperationResume,
			withCredential: true,
			expectedStatus: codespace_model.StatusStopped,
			expectKey:      true,
		},
		{
			name:           "stop",
			uuid:           "16161616-1616-4616-8616-161616161613",
			status:         codespace_model.StatusRunning,
			operationType:  codespace_model.OperationStop,
			withCredential: true,
			expectedStatus: codespace_model.StatusRunning,
			expectToken:    true,
			expectKey:      true,
		},
		{
			name:           "delete",
			uuid:           "16161616-1616-4616-8616-161616161614",
			status:         codespace_model.StatusDeleting,
			operationType:  codespace_model.OperationDelete,
			expectedStatus: codespace_model.StatusFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
				UUID:                 tc.uuid,
				Status:               tc.status,
				OperationRVersion:    40,
				OperationType:        tc.operationType,
				OperationStatus:      codespace_model.OperationStatusQueued,
				OperationTrigger:     codespace_model.OperationTriggerUser,
				OperationCreatedUnix: now - int64(setting.Codespace.QueueTimeout/time.Second) - 1,
			})
			if tc.withCredential {
				insertServiceCredentials(t, tc.uuid)
			}

			require.NoError(t, applyQueuedTimeout(t.Context(), loadServiceCodespace(t, tc.uuid), now))

			row := loadServiceCodespace(t, tc.uuid)
			assert.Equal(t, tc.expectedStatus, row.Status)
			assert.Empty(t, row.OperationType)
			assert.Empty(t, row.OperationStatus)
			assert.Empty(t, row.OperationTrigger)
			assert.Greater(t, row.UpdatedUnix, int64(1))
			if tc.expectToken {
				assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", tc.uuid)
			} else {
				assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", tc.uuid)
			}
			if tc.expectKey {
				assertServiceExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", tc.uuid)
			} else {
				assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", tc.uuid)
			}
		})
	}
}

func TestFetchOperationsRenewsObservedOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "20202020-2020-4020-8020-202020202020"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     32,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Second).Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		MaxOperations: 1,
		ObservedOperations: []ObservedOperation{{
			CodespaceUUID:     codespaceUUID,
			OperationRVersion: 32,
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	require.Len(t, result.RenewedLeases, 1)
	assert.Equal(t, codespaceUUID, result.RenewedLeases[0].CodespaceUUID)
	assert.EqualValues(t, 32, result.RenewedLeases[0].OperationRVersion)
	assert.EqualValues(t, setting.Codespace.OperationLeaseTimeout/time.Millisecond, result.RenewedLeases[0].LeaseValidForMilliseconds)
	assert.Greater(t, loadServiceCodespace(t, codespaceUUID).OperationDeadlineUnix, time.Now().Unix()+1)
}

func TestFetchOperationsRejectsStateHistoryConflictBeforeWrites(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	renewedUUID := "21212121-2121-4121-8121-212121212121"
	originalDeadline := time.Now().Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  renewedUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     36,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalDeadline,
	})
	conflictUUID := "22222222-2222-4222-8222-222222222221"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  conflictUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     37,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})

	_, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		MaxOperations: 1,
		ObservedOperations: []ObservedOperation{
			{CodespaceUUID: renewedUUID, OperationRVersion: 36},
			{CodespaceUUID: conflictUUID, OperationRVersion: 38},
		},
	})
	require.ErrorIs(t, err, ErrFetchStateHistoryConflict)
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, renewedUUID).OperationDeadlineUnix)
}

func TestFetchOperationsWaitsForUnobservedRunningOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "23232323-2323-4323-8323-232323232321"
	originalDeadline := time.Now().Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     38,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalDeadline,
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{MaxOperations: 1})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	assert.Empty(t, result.RenewedLeases)
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, codespaceUUID).OperationDeadlineUnix)
}

func TestFetchOperationsReturnsCurrentPayloadForLowerObservedVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "24242424-2424-4424-8424-242424242421"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     39,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		MaxOperations: 1,
		ObservedOperations: []ObservedOperation{{
			CodespaceUUID:     codespaceUUID,
			OperationRVersion: 38,
		}},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Empty(t, result.RenewedLeases)
	assert.Equal(t, codespaceUUID, result.Operations[0].CodespaceUUID)
	assert.EqualValues(t, 39, result.Operations[0].OperationRVersion)
	assert.Equal(t, codespace_model.OperationStop, result.Operations[0].Command)
	assert.Greater(t, loadServiceCodespace(t, codespaceUUID).OperationDeadlineUnix, time.Now().Unix()+1)
}

func TestFetchOperationsClaimsCleanupStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "30303030-3030-4030-8030-303030303030"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    33,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		MaxOperations:            1,
		CleanupCapacityAvailable: 1,
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Equal(t, codespace_model.OperationStop, result.Operations[0].Command)
	assert.Equal(t, codespaceUUID, result.Operations[0].CodespaceUUID)
	assert.Equal(t, codespace_model.OperationStatusRunning, loadServiceCodespace(t, codespaceUUID).OperationStatus)
}

func TestFetchOperationsRejectsStateHistoryConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "40404040-4040-4040-8040-404040404040"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     34,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})

	_, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		MaxOperations: 1,
		ObservedOperations: []ObservedOperation{{
			CodespaceUUID:     codespaceUUID,
			OperationRVersion: 35,
		}},
	})
	require.ErrorIs(t, err, ErrFetchStateHistoryConflict)
}

func markServiceManagerOnline(t *testing.T, manager *codespace_model.Manager, tagsJSON string) {
	t.Helper()
	manager.RuntimeState = codespace_model.ManagerRuntimeStateOnline
	manager.LastOnlineUnix = time.Now().Unix()
	manager.TagsJSON = tagsJSON
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state", "last_online_unix", "tags_json").Update(manager)
	require.NoError(t, err)
}
