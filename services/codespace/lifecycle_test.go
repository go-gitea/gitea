// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalizeOperationResumeFailedTransaction(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusStopped,
		OperationRVersion:     3,
		OperationType:         codespace_model.OperationResume,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	insertServiceCredentials(t, codespaceUUID)

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 3,
		OperationType:     codespacev1.OperationType_OPERATION_TYPE_RESUME,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_FAILED,
	})
	require.NoError(t, err)
	assert.False(t, outcome.GetResourceAbsent())

	codespace := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.StatusStopped, codespace.Status)
	assert.Empty(t, codespace.OperationType)
	assert.Empty(t, codespace.OperationStatus)
	assert.Positive(t, codespace.UpdatedUnix)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
}

func TestFinalizeOperationRejectsWrongManagerAsStale(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	codespaceUUID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     4,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := FinalizeOperation(t.Context(), otherManager, FinalizeOperationOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 4,
		OperationType:     codespacev1.OperationType_OPERATION_TYPE_STOP,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	assert.Equal(t, codespace_model.OperationStatusRunning, loadServiceCodespace(t, codespaceUUID).OperationStatus)
}

func TestFinalizeOperationStopTimeoutDoesNotMarkStopped(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbc"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     41,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(-time.Minute).Unix(),
	})
	insertServiceCredentials(t, codespaceUUID)

	_, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 41,
		OperationType:     codespacev1.OperationType_OPERATION_TYPE_STOP,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	codespace := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, codespace_model.StatusFailed, codespace.Status)
	assert.Empty(t, codespace.OperationType)
	assert.Empty(t, codespace.OperationStatus)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_uuid = ?", codespaceUUID)
	assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_uuid = ?", codespaceUUID)
	assert.Contains(t, readServiceLog(t, codespaceLogFilename(codespace.UUID)), "Gitea recorded operation stop#41 timeout as failed.")
}

func TestReportRuntimeMetadataRejectsStageRegression(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     5,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 5, "ready", nil),
		MetadataGeneration: 1,
	}))

	err := ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 5, "publish-ready", nil),
		MetadataGeneration: 2,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRuntimeMetadataStaleOperation)
	hasReady, err := HasReadyRuntimeMetadata(t.Context(), codespaceUUID, 5)
	require.NoError(t, err)
	assert.True(t, hasReady)
}

func TestReportRuntimeMetadataRejectsGenerationExhaustion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "cfcfcfcf-cfcf-4cfc-8cfc-cfcfcfcfcfcf"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     7,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	entry := serviceRuntimeMetadataEntry(t, 7, []map[string]any{})
	entry.Generation = math.MaxInt64
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, entry))

	err := ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 7, "ready", nil),
		MetadataGeneration: math.MaxInt64 - 1,
	})
	require.ErrorIs(t, err, ErrRuntimeMetadataVersionExhausted)
}

func TestReportRuntimeMetadataUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	staleManager := *manager
	codespaceUUID := "cececece-cece-4cec-8cec-cececececece"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     6,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	_, err := db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("last_online_unix").
		Update(&codespace_model.Manager{LastOnlineUnix: 1})
	require.NoError(t, err)

	err = ReportRuntimeMetadata(t.Context(), &staleManager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 6, "ready", nil),
		MetadataGeneration: 1,
	})
	require.ErrorIs(t, err, ErrRuntimeMetadataManagerOffline)
	hasReady, err := HasReadyRuntimeMetadata(t.Context(), codespaceUUID, 6)
	require.NoError(t, err)
	assert.False(t, hasReady)

	_, err = db.GetEngine(t.Context()).
		ID(manager.ID).
		Cols("runtime_state", "last_online_unix").
		Update(&codespace_model.Manager{
			RuntimeState:   codespace_model.ManagerRuntimeStateRecovering,
			LastOnlineUnix: time.Now().Unix(),
		})
	require.NoError(t, err)
	err = ReportRuntimeMetadata(t.Context(), &staleManager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 6, "ready", nil),
		MetadataGeneration: 1,
	})
	require.NoError(t, err)
}

func TestFinalizeOperationCreateDoneRejectsDamagedGiteaToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "11111111-1111-4111-8111-111111111111"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     6,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 6, "ready", nil),
		MetadataGeneration: 1,
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceUUID:  codespaceUUID,
		TokenHash:      "hash-" + codespaceUUID,
		TokenSalt:      "salt-1",
		TokenLastEight: "last0001",
		TokenEncrypted: "encrypted",
	}))

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 6,
		OperationType:     codespacev1.OperationType_OPERATION_TYPE_CREATE,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.ErrorIs(t, err, ErrFinalizeGiteaTokenRequired)
	assert.Empty(t, outcome)
	assert.Equal(t, codespace_model.OperationStatusRunning, loadServiceCodespace(t, codespaceUUID).OperationStatus)
}

func TestFinalizeStopClearsRuntimeMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     6,
		OperationType:         codespace_model.OperationStop,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	insertServiceCredentials(t, codespaceUUID)

	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceUUID:      codespaceUUID,
		Metadata:           serviceRuntimeMetadataProto(t, 6, "ready", nil),
		MetadataGeneration: 1,
	}))
	hasReady, err := HasReadyRuntimeMetadata(t.Context(), codespaceUUID, 6)
	require.NoError(t, err)
	require.True(t, hasReady)

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 6,
		OperationType:     codespacev1.OperationType_OPERATION_TYPE_STOP,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	assert.False(t, outcome.GetResourceAbsent())
	hasReady, err = HasReadyRuntimeMetadata(t.Context(), codespaceUUID, 6)
	require.NoError(t, err)
	assert.False(t, hasReady)
}

func insertServiceManager(t *testing.T) *codespace_model.Manager {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "manager",
		UserID:         0,
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       "[]",
		CreatedUnix:    1,
		LastOnlineUnix: 1,
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))
	return manager
}

func insertServiceCodespace(t *testing.T, managerID int64, codespace *codespace_model.Codespace) {
	t.Helper()
	codespace.ManagerID = managerID
	codespace.UserID = 1
	codespace.RepoID = 2
	codespace.RefType = "branch"
	codespace.RefName = "main"
	if codespace.EnvironmentTag == "" {
		codespace.EnvironmentTag = "default"
	}
	codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	codespace.DevContainerDefaultImage = "mcr.microsoft.com/devcontainers/base:ubuntu"
	if codespace.AutoStopMode == "" {
		codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	}
	codespace.CreatedUnix = 1
	codespace.UpdatedUnix = 1
	require.NoError(t, db.Insert(t.Context(), codespace))
}

func insertServiceCredentials(t *testing.T, codespaceUUID string) {
	t.Helper()
	_, err := insertNewGiteaToken(t.Context(), codespaceUUID)
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.SSHKey{
		CodespaceUUID: codespaceUUID,
		KeyID:         time.Now().UnixNano(),
	}))
}

func loadServiceCodespace(t *testing.T, codespaceUUID string) *codespace_model.Codespace {
	t.Helper()
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).ID(codespaceUUID).Get(codespace)
	require.NoError(t, err)
	require.True(t, has)
	return codespace
}

func assertServiceExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.True(t, has)
}

func assertServiceNotExists(t *testing.T, bean any, query string, args ...any) {
	t.Helper()
	has, err := db.GetEngine(t.Context()).Where(query, args...).Exist(bean)
	require.NoError(t, err)
	assert.False(t, has)
}
