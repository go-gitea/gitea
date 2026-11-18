// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchOperationsClaimsCreate(t *testing.T) {
	for _, source := range []struct {
		name    string
		path    string
		content string
	}{
		{name: "repository", path: ".devcontainer/devcontainer.json"},
		{name: "template", content: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`},
	} {
		t.Run(source.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

			fullManager := insertServiceManager(t)
			markServiceManagerOnline(t, fullManager, `[{"tag":"default"}]`)
			manager := insertServiceManager(t)
			markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
			empty, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{})
			require.NoError(t, err)
			assert.Empty(t, empty.Operations)
			assert.Empty(t, empty.RenewedLeases)
			codespaceID := int64(1014)
			insertServiceCodespace(t, 0, &codespace_model.Codespace{
				ID:                   codespaceID,
				DevContainerPath:     source.path,
				DevContainerContent:  source.content,
				Status:               codespace_model.StatusCreating,
				OperationRVersion:    31,
				OperationTrigger:     codespace_model.OperationTriggerUser,
				OperationCreatedUnix: time.Now().Unix(),
			})

			fullResult, err := FetchOperations(t.Context(), fullManager, FetchOperationsOptions{
				AcceptedCreateTags: []string{"default"},
			})
			require.NoError(t, err)
			assert.Empty(t, fullResult.Operations)
			waiting := loadServiceCodespace(t, codespaceID)
			assert.Zero(t, waiting.ManagerID)
			assert.True(t, codespace_model.IsOperationQueued(waiting))

			result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
				StartupCapacityAvailable: 1,
				AcceptedCreateTags:       []string{"default"},
			})
			require.NoError(t, err)
			require.Len(t, result.Operations, 1)
			operation := result.Operations[0]
			assert.Equal(t, codespaceID, operation.GetCodespaceId())
			assert.EqualValues(t, 31, operation.GetOperationRversion())
			assert.EqualValues(t, operationLeaseDuration/time.Millisecond, operation.GetLeaseValidForMilliseconds())
			create := operation.GetCreate()
			require.NotNil(t, create)
			assert.Equal(t, source.path, create.GetDevContainer().GetRepositoryPath())
			assert.Equal(t, source.content, create.GetDevContainer().GetTemplateContent())
			repository := create.GetRepository()
			require.NotNil(t, repository)
			assert.NotEmpty(t, repository.GetFullName())
			require.Len(t, repository.GetCloneUrls(), 1)
			assert.Equal(t, "refs/heads/main", repository.GetStartRef())
			assert.Contains(t, repository.GetCloneUrls()[0], "/user2/repo2.git")
			assert.EqualValues(t, setting.Codespace.AutoStopDefaultTimeout/time.Second, create.GetRuntimeSettings().GetIdleTimeoutSeconds())
			row := loadServiceCodespace(t, codespaceID)
			assert.Equal(t, row.RepoID, repository.GetRepositoryId())
			assert.Equal(t, row.UserID, create.GetGitIdentity().GetUserId())
			assert.Equal(t, manager.ID, row.ManagerID)
			assert.True(t, codespace_model.IsOperationRunning(row))
			assert.Positive(t, row.OperationStartedUnix)
			assert.Positive(t, row.OperationDeadlineUnix)
		})
	}
}

func TestFetchOperationsReleasesCreateClaimWhenPayloadInvalid(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1015)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   codespaceID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    32,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err := db.GetEngine(t.Context()).Where("id = ?", codespaceID).Cols("dev_container_path").Update(&codespace_model.Codespace{DevContainerPath: ".devcontainer/devcontainer.json"})
	require.NoError(t, err)

	_, err = FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedCreateTags:       []string{"default"},
	})
	require.Error(t, err)
	row := loadServiceCodespace(t, codespaceID)
	assert.Zero(t, row.ManagerID)
	assert.True(t, codespace_model.IsOperationQueued(row))
	assert.Zero(t, row.OperationStartedUnix)
	assert.Zero(t, row.OperationDeadlineUnix)
}

func TestFetchOperationsClaimsOnlyAcceptedCreateTag(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"},{"tag":"gpu"}]`)
	for i, tag := range []string{"default", "gpu"} {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{
			ID:                   []int64{int64(1016), int64(1017)}[i],
			EnvironmentTag:       tag,
			Status:               codespace_model.StatusCreating,
			OperationRVersion:    1,
			OperationTrigger:     codespace_model.OperationTriggerUser,
			OperationCreatedUnix: time.Now().Unix(),
		})
	}

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedCreateTags:       []string{"gpu"},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Equal(t, "gpu", result.Operations[0].GetCreate().GetEnvironmentTag())
}

func TestFetchOperationsResumeUsesExistingManagerBinding(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"gpu"}]`)
	codespaceID := int64(1018)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   codespaceID,
		EnvironmentTag:       "default",
		Status:               codespace_model.StatusStopped,
		OperationRVersion:    2,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Equal(t, codespaceID, result.Operations[0].GetCodespaceId())
	assert.NotNil(t, result.Operations[0].GetResume())
}

func TestCanonicalStartRef(t *testing.T) {
	testCases := []struct {
		refType string
		refName string
		want    string
	}{
		{"branch", "feature/editor", "refs/heads/feature/editor"},
		{"tag", "v1.0.0", "refs/tags/v1.0.0"},
		{"pull", "refs/pull/42/head", "refs/pull/42/head"},
		{"commit", "0123456789abcdef0123456789abcdef01234567", ""},
	}
	for _, testCase := range testCases {
		got, err := canonicalStartRef(testCase.refType, testCase.refName)
		require.NoError(t, err)
		assert.Equal(t, testCase.want, got)
	}
}

func TestBuildCreatePayloadUsesPullRequestHeadBranch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

	payload, err := buildCreatePayload(t.Context(), &codespace_model.Codespace{
		UserID:              2,
		RepoID:              1,
		RefType:             "pull",
		RefName:             "refs/pull/3/head",
		CommitSHA:           "985f0301dba5e7b34be866819cd15ad3d8f508ee",
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		AutoStopMode:        codespace_model.AutoStopModeDefault,
	})
	require.NoError(t, err)
	require.NotNil(t, payload.GetRepository())
	assert.Equal(t, "user2/repo1", payload.GetRepository().GetFullName())
	assert.Contains(t, payload.GetRepository().GetCloneUrls()[0], "/user2/repo1.git")
	assert.Equal(t, "refs/heads/branch2", payload.GetRepository().GetStartRef())
}

func TestBuildCreatePayloadUsesForkPullRequestRepository(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	require.NoError(t, db.Insert(t.Context(), &git_model.Branch{RepoID: 11, Name: "branch2"}))
	configureGitTransportTestSettings(t, codespace_model.GitProtocolHTTP, false, false, nil)

	payload, err := buildCreatePayload(t.Context(), &codespace_model.Codespace{
		UserID:              11,
		RepoID:              10,
		RefType:             "pull",
		RefName:             "refs/pull/1/head",
		CommitSHA:           "0abcb056019adb83",
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		AutoStopMode:        codespace_model.AutoStopModeDefault,
	})
	require.NoError(t, err)
	require.NotNil(t, payload.GetRepository())
	assert.Equal(t, "user12/repo10", payload.GetRepository().GetFullName())
	assert.Contains(t, payload.GetRepository().GetCloneUrls()[0], "/user13/repo11.git")
	assert.Equal(t, "refs/heads/branch2", payload.GetRepository().GetStartRef())
}

func TestFetchOperationsReturnsSSHCloneURLWhenEnabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureGitTransportTestSettings(t, codespace_model.GitProtocolSSH, false, false, []string{
		"gitea.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf",
	})

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1019)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   codespaceID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    32,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedCreateTags:       []string{"default"},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	create := result.Operations[0].GetCreate()
	require.NotNil(t, create)
	require.NotNil(t, create.GetRepository())
	require.Len(t, create.GetRepository().GetCloneUrls(), 2)
	assert.Contains(t, create.GetRepository().GetCloneUrls()[0], "sshuser@")
}

func TestFetchOperationsSkipsCreateOutsideManagerUserScope(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	manager.UserID = 2
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("user_id").Update(manager)
	require.NoError(t, err)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1020)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   codespaceID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    35,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedCreateTags:       []string{"default"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	row := loadServiceCodespace(t, codespaceID)
	assert.Zero(t, row.ManagerID)
	assert.True(t, codespace_model.IsOperationQueued(row))
}

func TestFetchOperationsDisabledDrainsWithoutClaimingStartup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.Enabled, false))

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	runningCreateID := int64(1021)
	runningStopID := int64(1022)
	queuedCreateID := int64(1023)
	queuedStopID := int64(1024)
	now := time.Now()
	originalCreateDeadline := now.Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningCreateID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     51,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  now.Add(-time.Minute).Unix(),
		OperationStartedUnix:  now.Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalCreateDeadline,
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    runningStopID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     52,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  now.Add(-time.Minute).Unix(),
		OperationStartedUnix:  now.Add(-time.Minute).Unix(),
		OperationDeadlineUnix: now.Add(time.Minute).Unix(),
	})
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   queuedCreateID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    53,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now.Unix(),
	})
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   queuedStopID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    54,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: now.Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedCreateTags:       []string{"default"},
		CleanupCapacityAvailable: 1,
		ObservedOperations: []*codespacev1.ObservedOperation{
			{CodespaceId: runningCreateID, OperationRversion: 51},
			{CodespaceId: runningStopID, OperationRversion: 52},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 2)
	assert.NotNil(t, result.Operations[0].GetAbortCreate())
	assert.Zero(t, result.Operations[0].GetLeaseValidForMilliseconds())
	assert.NotNil(t, result.Operations[1].GetStop())
	require.Len(t, result.RenewedLeases, 1)
	assert.Equal(t, runningStopID, result.RenewedLeases[0].GetCodespaceId())

	assert.Equal(t, originalCreateDeadline, loadServiceCodespace(t, runningCreateID).OperationDeadlineUnix)
	assert.Zero(t, loadServiceCodespace(t, queuedCreateID).ManagerID)
	assert.True(t, codespace_model.IsOperationQueued(loadServiceCodespace(t, queuedCreateID)))
	assert.True(t, codespace_model.IsOperationRunning(loadServiceCodespace(t, queuedStopID)))
}

func TestFetchOperationsRenewsObservedOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1025)
	now := time.Now()
	originalDeadline := now.Add(operationLeaseDuration / 4).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     32,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  now.Add(-time.Minute).Unix(),
		OperationStartedUnix:  now.Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalDeadline,
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		ObservedOperations: []*codespacev1.ObservedOperation{{
			CodespaceId:       codespaceID,
			OperationRversion: 32,
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	require.Len(t, result.RenewedLeases, 1)
	assert.Equal(t, codespaceID, result.RenewedLeases[0].GetCodespaceId())
	assert.EqualValues(t, 32, result.RenewedLeases[0].GetOperationRversion())
	assert.EqualValues(t, operationLeaseDuration/time.Millisecond, result.RenewedLeases[0].GetLeaseValidForMilliseconds())
	deadline := loadServiceCodespace(t, codespaceID).OperationDeadlineUnix
	assert.Greater(t, deadline, originalDeadline)

	_, err = db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(&codespace_model.Manager{RuntimeState: codespace_model.ManagerRuntimeStateRecovering})
	require.NoError(t, err)
	queuedID := int64(1026)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID: queuedID, Status: codespace_model.StatusRunning,
		OperationRVersion: 1, OperationTrigger: codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	fetchStarted := time.Now()
	result, err = FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1, CleanupCapacityAvailable: 1,
		AcceptedCreateTags: []string{"default"},
		ObservedOperations: []*codespacev1.ObservedOperation{{CodespaceId: codespaceID, OperationRversion: 32}},
	})
	fetchFinished := time.Now()
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	require.Len(t, result.RenewedLeases, 1)
	assert.True(t, codespace_model.IsOperationQueued(loadServiceCodespace(t, queuedID)))
	assert.Equal(t, deadline, loadServiceCodespace(t, codespaceID).OperationDeadlineUnix)
	leaseMillis := result.RenewedLeases[0].GetLeaseValidForMilliseconds()
	assert.GreaterOrEqual(t, leaseMillis, time.Unix(deadline, 0).Sub(fetchFinished).Milliseconds())
	assert.LessOrEqual(t, leaseMillis, time.Unix(deadline, 0).Sub(fetchStarted).Milliseconds())
}

func TestFetchOperationsRejectsStateHistoryConflictBeforeWrites(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	renewedID := int64(1027)
	originalDeadline := time.Now().Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    renewedID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     36,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalDeadline,
	})
	conflictID := int64(1028)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    conflictID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     37,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})

	_, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		ObservedOperations: []*codespacev1.ObservedOperation{
			{CodespaceId: renewedID, OperationRversion: 36},
			{CodespaceId: conflictID, OperationRversion: 38},
		},
	})
	require.ErrorIs(t, err, ErrFetchStateHistoryConflict)
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, renewedID).OperationDeadlineUnix)
}

func TestFetchOperationsRedeliversUnobservedRunningOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1029)
	originalDeadline := time.Now().Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     38,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalDeadline,
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Empty(t, result.RenewedLeases)
	assert.Equal(t, codespaceID, result.Operations[0].GetCodespaceId())
	assert.EqualValues(t, 38, result.Operations[0].GetOperationRversion())
	assert.Positive(t, result.Operations[0].GetLeaseValidForMilliseconds())
	assert.LessOrEqual(t, result.Operations[0].GetLeaseValidForMilliseconds(), int64(time.Minute/time.Millisecond))
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, codespaceID).OperationDeadlineUnix)
}

func TestGrantLease(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, tc := range []struct {
		name     string
		maximum  time.Duration
		started  int64
		expected time.Duration
	}{
		{name: "initial lease", maximum: 2 * time.Hour, expected: 5 * time.Minute},
		{name: "short operation", maximum: time.Minute, expected: time.Minute},
		{name: "renewal near deadline", maximum: 2 * time.Hour, started: now.Add(-2*time.Hour + 20*time.Second).Unix(), expected: 20 * time.Second},
		{name: "expired operation", maximum: 2 * time.Hour, started: now.Add(-2 * time.Hour).Unix()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(test.MockVariableValue(&setting.Codespace.OperationMaxDuration, tc.maximum))
			milliseconds, deadline, ok := grantLease(tc.started, now)
			assert.Equal(t, tc.expected > 0, ok)
			assert.Equal(t, tc.expected.Milliseconds(), milliseconds)
			if ok {
				assert.Equal(t, now.Add(tc.expected).Unix(), deadline)
			} else {
				assert.Zero(t, deadline)
			}
		})
	}
}

func TestFetchOperationsExpiresRunningOperation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		started  time.Duration
		deadline time.Duration
		observed bool
	}{
		{name: "unobserved lease expired", started: -time.Minute, deadline: -time.Second},
		{name: "observed total duration expired", started: -setting.Codespace.OperationMaxDuration - time.Minute, deadline: 2 * time.Second, observed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			manager := insertServiceManager(t)
			markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
			codespaceID := int64(1030)
			now := time.Now()
			insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
				ID:                    codespaceID,
				Status:                codespace_model.StatusCreating,
				OperationRVersion:     40,
				OperationTrigger:      codespace_model.OperationTriggerUser,
				OperationCreatedUnix:  now.Add(tc.started).Unix(),
				OperationStartedUnix:  now.Add(tc.started).Unix(),
				OperationDeadlineUnix: now.Add(tc.deadline).Unix(),
			})
			opts := FetchOperationsOptions{}
			if tc.observed {
				opts.ObservedOperations = []*codespacev1.ObservedOperation{{CodespaceId: codespaceID, OperationRversion: 40}}
			}
			result, err := FetchOperations(t.Context(), manager, opts)
			require.NoError(t, err)
			assert.Empty(t, result.Operations)
			assert.Empty(t, result.RenewedLeases)
			current := loadServiceCodespace(t, codespaceID)
			assert.Equal(t, codespace_model.StatusFailed, current.Status)
			assert.False(t, hasActiveOperation(current))
			assert.Contains(t, readServiceLog(t, codespaceLogFilename(codespaceID)), "operation create#40 timeout as failed")
		})
	}
}

func TestFetchOperationsReturnsCurrentPayloadForLowerObservedVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1031)
	originalDeadline := time.Now().Add(time.Minute).Unix()
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusRunning,
		OperationRVersion:     39,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationStartedUnix:  time.Now().Add(-time.Minute).Unix(),
		OperationDeadlineUnix: originalDeadline,
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		ObservedOperations: []*codespacev1.ObservedOperation{{
			CodespaceId:       codespaceID,
			OperationRversion: 38,
		}},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Empty(t, result.RenewedLeases)
	assert.Equal(t, codespaceID, result.Operations[0].GetCodespaceId())
	assert.EqualValues(t, 39, result.Operations[0].GetOperationRversion())
	assert.NotNil(t, result.Operations[0].GetStop())
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, codespaceID).OperationDeadlineUnix)
}

func TestFetchOperationsClaimsCleanupStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1026)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   codespaceID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    33,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		CleanupCapacityAvailable: 1,
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.NotNil(t, result.Operations[0].GetStop())
	assert.Equal(t, codespaceID, result.Operations[0].GetCodespaceId())
	assert.True(t, codespace_model.IsOperationRunning(loadServiceCodespace(t, codespaceID)))
}

func TestFetchOperationsRejectsStateHistoryConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1032)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                    codespaceID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     34,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  time.Now().Unix(),
		OperationStartedUnix:  time.Now().Unix(),
		OperationDeadlineUnix: time.Now().Add(time.Minute).Unix(),
	})

	_, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		ObservedOperations: []*codespacev1.ObservedOperation{{
			CodespaceId:       codespaceID,
			OperationRversion: 35,
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
