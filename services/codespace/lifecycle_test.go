// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"math"
	"strconv"
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
	codespaceID := int64(1033)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusStopped,
		OperationRVersion:     3,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	insertServiceCredentials(t, codespaceID)

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 3,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_FAILED,
	})
	require.NoError(t, err)
	assert.False(t, outcome.GetResourceAbsent())

	codespace := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.StatusStopped, codespace.Status)
	assert.False(t, hasActiveOperation(codespace))
	assert.Positive(t, codespace.UpdatedUnix)
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = ?", codespaceID)
}

func TestFinalizeOperationCreateFailureWaitsForCleanupInventory(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespace := &codespace_model.Codespace{
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     3,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	}
	insertServiceCodespace(t, manager.ID, codespace)

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespace.ID,
		OperationRVersion: 3,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_FAILED,
	})
	require.NoError(t, err)
	assert.False(t, outcome.GetResourceAbsent())

	current := loadServiceCodespace(t, codespace.ID)
	assert.Equal(t, codespace_model.StatusFailed, current.Status)
	assert.Equal(t, manager.ID, current.ManagerID)
	assert.False(t, hasActiveOperation(current))
	_, err = ReportInstances(t.Context(), manager, ReportInstancesOptions{InventoryGeneration: 1})
	require.NoError(t, err)
	assert.Zero(t, loadServiceCodespace(t, codespace.ID).ManagerID)
}

func TestFinalizeOperationCreateUsesDurablePrerequisites(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1126)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     3,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	insertServiceCredentials(t, codespaceID)

	_, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 3,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	row := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.StatusRunning, row.Status)
	assert.EqualValues(t, 3, row.RuntimeOperationRVersion)
	_, hasMetadata, err := getRuntimeMetadataEntry(codespaceID)
	require.NoError(t, err)
	assert.False(t, hasMetadata)
}

func TestFinalizeOperationRejectsWrongManagerAsStale(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	codespaceID := int64(1127)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     4,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := FinalizeOperation(t.Context(), otherManager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 4,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	assert.True(t, codespace_model.IsOperationRunning(loadServiceCodespace(t, codespaceID)))
}

func TestFinalizeOperationAcceptsConvergentCompletionAfterDeadline(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1128)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     41,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(-time.Minute).Unix(),
	})
	insertServiceCredentials(t, codespaceID)

	_, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 41,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	codespace := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.StatusStopped, codespace.Status)
	assert.False(t, hasActiveOperation(codespace))
	assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = ?", codespaceID)
	assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = ?", codespaceID)
}

func TestFinalizeOperationRejectsConvergentFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager := insertServiceManager(t)
	codespaceID := int64(1129)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID: codespaceID, Status: codespace_model.StatusDeleting, OperationRVersion: 42,
		OperationTrigger: codespace_model.OperationTriggerUser, OperationCreatedUnix: 10,
		OperationStartedUnix: 11, OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID: codespaceID, OperationRVersion: 42,
		FinalStatus: codespacev1.FinalStatus_FINAL_STATUS_FAILED,
	})
	require.Error(t, err)
	row := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.StatusDeleting, row.Status)
	assert.True(t, codespace_model.IsOperationRunning(row))
}

func TestReportRuntimeMetadataRejectsStageRegression(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1130)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     5,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(5, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: 1,
	}))

	err := ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(5, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY, nil),
		MetadataGeneration: 2,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRuntimeMetadataStaleOperation)
	hasReady, err := hasReadyRuntimeMetadataForTest(codespaceID, 5)
	require.NoError(t, err)
	assert.True(t, hasReady)
}

func TestReportRuntimeMetadataResourceUsageRefresh(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1131)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID: codespaceID, Status: codespace_model.StatusCreating,
		OperationRVersion: 5, OperationTrigger: codespace_model.OperationTriggerUser,
		OperationCreatedUnix: 10, OperationStartedUnix: 11, OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	metadata := serviceRuntimeMetadataProto(5, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil)
	metadata.ResourceUsage = nil
	options := ReportRuntimeMetadataOptions{CodespaceID: codespaceID, Metadata: metadata, MetadataGeneration: 1}
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, options))
	ready, err := hasReadyRuntimeMetadataForTest(codespaceID, 5)
	require.NoError(t, err)
	assert.True(t, ready)

	metadata.ResourceUsage = serviceRuntimeMetadataResourceUsage()
	metadata.ResourceUsage.ObservedUnix = time.Now().Unix() - 10
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, options))
	metadata.ResourceUsage.ObservedUnix++
	metadata.ResourceUsage.Cpu.UsedMillicores = 250
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, options))
	metadata.ResourceUsage.ObservedUnix--
	metadata.ResourceUsage.Cpu.UsedMillicores = 100
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, options))
	entry, exists, err := getRuntimeMetadataEntry(codespaceID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.EqualValues(t, 1, entry.Generation)
	assert.EqualValues(t, 250, entry.Metadata.ResourceUsage.CPU.UsedMillicores)
	assert.NotNil(t, creatorResourceUsageView(entry.Metadata.ResourceUsage))

	entry.Metadata.ResourceUsage.ObservedUnix = time.Now().Add(-runtimeMetadataMaxAge - time.Second).Unix()
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, entry))
	metadata.ResourceUsage = nil
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, options))
	entry, exists, err = getRuntimeMetadataEntry(codespaceID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Nil(t, creatorResourceUsageView(entry.Metadata.ResourceUsage))
	ready, err = hasReadyRuntimeMetadataForTest(codespaceID, 5)
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestReportRuntimeMetadataAddresses(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	cs := &codespace_model.Codespace{Status: codespace_model.StatusRunning, OperationRVersion: 5}
	insertServiceCodespace(t, manager.ID, cs)
	metadata := serviceRuntimeMetadataProto(5, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil)
	opts := ReportRuntimeMetadataOptions{CodespaceID: cs.ID, Metadata: metadata, MetadataGeneration: 1}
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, opts))

	metadata.Endpoints[0].Url = "https://new.gateway.example.com/"
	require.ErrorIs(t, ReportRuntimeMetadata(t.Context(), manager, opts), ErrRuntimeMetadataGenerationConflict)
	opts.MetadataGeneration++
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, opts))
	entry, exists, err := getRuntimeMetadataEntry(cs.ID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, metadata.Endpoints[0].Url, entry.Metadata.Endpoints[0].URL)

	opts.MetadataGeneration++
	for _, address := range []string{"http://new.gateway.example.com/", "https://outside.example.com/"} {
		metadata.Endpoints[0].Url = address
		require.Error(t, ReportRuntimeMetadata(t.Context(), manager, opts))
	}
	metadata.Endpoints[0].Url = "https://new.gateway.example.com/"
	for _, address := range []string{
		"https://reported-runtime@ssh.example.com:2222",
		"ssh://reported-runtime@outside.example.com:2222",
		"ssh://reported-runtime:password@ssh.example.com:2222",
		"ssh://reported-runtime@ssh.example.com:0",
		"ssh://reported-runtime@ssh.example.com:2222/workspaces/project",
	} {
		metadata.Ssh = address
		require.Error(t, ReportRuntimeMetadata(t.Context(), manager, opts))
	}
}

func TestReportRuntimeMetadataRejectsGenerationExhaustion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1132)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     7,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	entry := serviceRuntimeMetadataEntry(t, 7, nil)
	entry.Generation = math.MaxInt64
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, entry))

	err := ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(7, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: math.MaxInt64 - 1,
	})
	require.ErrorIs(t, err, ErrRuntimeMetadataVersionExhausted)
}

func TestReportRuntimeMetadataUsesCurrentManagerAvailability(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	staleManager := *manager
	codespaceID := int64(1133)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     6,
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
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(6, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: 1,
	})
	require.ErrorIs(t, err, ErrRuntimeMetadataManagerOffline)
	hasReady, err := hasReadyRuntimeMetadataForTest(codespaceID, 6)
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
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(6, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: 1,
	})
	require.NoError(t, err)
}

func TestFinalizeOperationCreateDoneRejectsDamagedGiteaToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1134)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     6,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(6, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: 1,
	}))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.GiteaToken{
		CodespaceID:    loadServiceCodespace(t, codespaceID).ID,
		TokenHash:      "hash-" + strconv.FormatInt(codespaceID, 10),
		TokenSalt:      "salt-1",
		TokenLastEight: "last0001",
		TokenEncrypted: "encrypted",
	}))

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 6,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.ErrorIs(t, err, ErrFinalizeGiteaTokenRequired)
	assert.Empty(t, outcome)
	assert.True(t, codespace_model.IsOperationRunning(loadServiceCodespace(t, codespaceID)))
}

func TestFinalizeStopClearsRuntimeMetadata(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1135)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     6,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	insertServiceCredentials(t, codespaceID)

	require.NoError(t, ReportRuntimeMetadata(t.Context(), manager, ReportRuntimeMetadataOptions{
		CodespaceID:        codespaceID,
		Metadata:           serviceRuntimeMetadataProto(6, codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY, nil),
		MetadataGeneration: 1,
	}))
	hasReady, err := hasReadyRuntimeMetadataForTest(codespaceID, 6)
	require.NoError(t, err)
	require.True(t, hasReady)

	outcome, err := FinalizeOperation(t.Context(), manager, FinalizeOperationOptions{
		CodespaceID:       codespaceID,
		OperationRVersion: 6,
		FinalStatus:       codespacev1.FinalStatus_FINAL_STATUS_DONE,
	})
	require.NoError(t, err)
	assert.False(t, outcome.GetResourceAbsent())
	hasReady, err = hasReadyRuntimeMetadataForTest(codespaceID, 6)
	require.NoError(t, err)
	assert.False(t, hasReady)
}

func TestLifecycleTransitionDoesNotConflictWithUserActivity(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceID := int64(1131)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     7,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
		InteractionGeneration: 2,
	})
	staleInteraction := loadServiceCodespace(t, codespaceID)
	updated, err := db.GetEngine(t.Context()).ID(staleInteraction.ID).
		Cols("interaction_generation").
		Update(&codespace_model.Codespace{InteractionGeneration: 3})
	require.NoError(t, err)
	require.EqualValues(t, 1, updated)

	require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		return applyFinalState(ctx, staleInteraction, codespace_model.StatusStopped, time.Now().Unix())
	}))
	current := loadServiceCodespace(t, codespaceID)
	assert.Equal(t, codespace_model.StatusStopped, current.Status)
	assert.EqualValues(t, 3, current.InteractionGeneration)
}

func insertServiceManager(t *testing.T) *codespace_model.Manager {
	t.Helper()
	manager := &codespace_model.Manager{
		Name:           "manager",
		GatewayURL:     "https://gateway.example.com",
		GatewaySSHAddr: "ssh.example.com:2222",
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
	if codespace.Status == codespace_model.StatusRunning && codespace.RuntimeOperationRVersion == 0 {
		codespace.RuntimeOperationRVersion = codespace.OperationRVersion
	}
	if codespace.UserID == 0 {
		codespace.UserID = 1
	}
	codespace.RepoID = 2
	codespace.RefType = "branch"
	codespace.RefName = "main"
	if codespace.EnvironmentTag == "" {
		codespace.EnvironmentTag = "default"
	}
	codespace.CommitSHA = "0123456789abcdef0123456789abcdef01234567"
	if codespace.DevContainerPath == "" && codespace.DevContainerContent == "" {
		codespace.DevContainerContent = `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`
	}
	if codespace.AutoStopMode == "" {
		codespace.AutoStopMode = codespace_model.AutoStopModeDefault
	}
	codespace.CreatedUnix = 1
	codespace.UpdatedUnix = 1
	err := db.Insert(t.Context(), codespace)
	require.NoError(t, err)
}

func hasReadyRuntimeMetadataForTest(codespaceID, operationRVersion int64) (bool, error) {
	entry, has, err := getRuntimeMetadataEntry(codespaceID)
	if err != nil || !has {
		return false, err
	}
	return entry.Metadata.Boot.Stage == codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY && entry.Metadata.Boot.OperationRVersion == operationRVersion, nil
}

func insertServiceCredentials(t *testing.T, codespaceID int64) {
	t.Helper()
	_, err := insertNewGiteaToken(t.Context(), codespaceID)
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &codespace_model.SSHKey{
		CodespaceID: codespaceID,
		KeyID:       time.Now().UnixNano(),
	}))
}

func loadServiceCodespace(t *testing.T, codespaceID int64) *codespace_model.Codespace {
	t.Helper()
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(t.Context()).Where("id = ?", codespaceID).Get(codespace)
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
