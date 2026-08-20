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
	require.NoError(t, unittest.PrepareTestDatabase())
	configureFetchGitTransport(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE},
		AcceptedCreateTags:       []string{"default"},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	operation := result.Operations[0]
	assert.Equal(t, codespaceUUID, operation.GetRuntimeUuid())
	assert.EqualValues(t, 31, operation.GetOperationRversion())
	assert.EqualValues(t, setting.Codespace.OperationLeaseTimeout/time.Millisecond, operation.GetLeaseValidForMilliseconds())
	create := operation.GetCreate()
	require.NotNil(t, create)
	repository := create.GetRepository()
	require.NotNil(t, repository)
	assert.NotEmpty(t, repository.GetFullName())
	assert.NotEmpty(t, repository.GetCloneHttpUrl())
	assert.Empty(t, repository.GetCloneSshUrl())
	assert.Equal(t, "refs/heads/main", repository.GetStartRef())
	assert.Equal(t, codespacev1.GitProtocol_GIT_PROTOCOL_HTTP, repository.GetPreferredProtocol())
	assert.True(t, create.GetRuntimeSettings().GetAutoStopEnabled())
	assert.EqualValues(t, setting.Codespace.AutoStopDefaultTimeout/time.Second, create.GetRuntimeSettings().GetIdleTimeoutSeconds())
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, manager.ID, row.ManagerID)
	assert.Equal(t, codespace_model.OperationStatusRunning, row.OperationStatus)
	assert.Positive(t, row.OperationStartedUnix)
	assert.Positive(t, row.OperationDeadlineUnix)
}

func TestFetchOperationsReleasesCreateClaimWhenPayloadInvalid(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureFetchGitTransport(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "10101010-1010-4010-8010-101010101011"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    32,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})
	_, err := db.GetEngine(t.Context()).Where("uuid = ?", codespaceUUID).Cols("dev_container_path").Update(&codespace_model.Codespace{DevContainerPath: ".devcontainer/devcontainer.json"})
	require.NoError(t, err)

	_, err = FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE},
		AcceptedCreateTags:       []string{"default"},
	})
	require.Error(t, err)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Zero(t, row.ManagerID)
	assert.Equal(t, codespace_model.OperationStatusQueued, row.OperationStatus)
	assert.Zero(t, row.OperationStartedUnix)
	assert.Zero(t, row.OperationDeadlineUnix)
}

func TestFetchOperationsClaimsOnlyAcceptedCreateTag(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureFetchGitTransport(t, codespace_model.GitProtocolHTTP, false, false, nil)

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"},{"tag":"gpu"}]`)
	for i, tag := range []string{"default", "gpu"} {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{
			UUID:                 []string{"11111111-1010-4010-8010-101010101010", "22222222-1010-4010-8010-101010101010"}[i],
			EnvironmentTag:       tag,
			Status:               codespace_model.StatusCreating,
			OperationRVersion:    1,
			OperationType:        codespace_model.OperationCreate,
			OperationStatus:      codespace_model.OperationStatusQueued,
			OperationTrigger:     codespace_model.OperationTriggerUser,
			OperationCreatedUnix: time.Now().Unix(),
		})
	}

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE},
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
	codespaceUUID := "33333333-1010-4010-8010-101010101010"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		EnvironmentTag:       "default",
		Status:               codespace_model.StatusStopped,
		OperationRVersion:    2,
		OperationType:        codespace_model.OperationResume,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Equal(t, codespaceUUID, result.Operations[0].GetRuntimeUuid())
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
	configureFetchGitTransport(t, codespace_model.GitProtocolHTTP, false, false, nil)

	payload, err := buildCreatePayload(t.Context(), &codespace_model.Codespace{
		UserID:              2,
		RepoID:              1,
		RefType:             "pull",
		RefName:             "refs/pull/3/head",
		CommitSHA:           "985f0301dba5e7b34be866819cd15ad3d8f508ee",
		DevContainerSource:  codespace_model.DevContainerSourceTemplate,
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		AutoStopMode:        codespace_model.AutoStopModeDefault,
	})
	require.NoError(t, err)
	require.NotNil(t, payload.GetRepository())
	assert.Equal(t, "user2/repo1", payload.GetRepository().GetFullName())
	assert.Contains(t, payload.GetRepository().GetCloneHttpUrl(), "/user2/repo1.git")
	assert.Equal(t, "refs/heads/branch2", payload.GetRepository().GetStartRef())
}

func TestBuildCreatePayloadUsesForkPullRequestRepository(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	require.NoError(t, db.Insert(t.Context(), &git_model.Branch{RepoID: 11, Name: "branch2"}))
	configureFetchGitTransport(t, codespace_model.GitProtocolHTTP, false, false, nil)

	payload, err := buildCreatePayload(t.Context(), &codespace_model.Codespace{
		UserID:              11,
		RepoID:              10,
		RefType:             "pull",
		RefName:             "refs/pull/1/head",
		CommitSHA:           "0abcb056019adb83",
		DevContainerSource:  codespace_model.DevContainerSourceTemplate,
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		AutoStopMode:        codespace_model.AutoStopModeDefault,
	})
	require.NoError(t, err)
	require.NotNil(t, payload.GetRepository())
	assert.Equal(t, "user12/repo10", payload.GetRepository().GetFullName())
	assert.Contains(t, payload.GetRepository().GetCloneHttpUrl(), "/user13/repo11.git")
	assert.Equal(t, "refs/heads/branch2", payload.GetRepository().GetStartRef())
}

func TestFetchOperationsReturnsSSHCloneURLWhenEnabled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	configureFetchGitTransport(t, codespace_model.GitProtocolSSH, false, false, []string{
		"localhost ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf",
	})

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "12121212-1212-4212-8212-121212121212"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusCreating,
		OperationRVersion:    32,
		OperationType:        codespace_model.OperationCreate,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: time.Now().Unix(),
	})

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE},
		AcceptedCreateTags:       []string{"default"},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	create := result.Operations[0].GetCreate()
	require.NotNil(t, create)
	require.NotNil(t, create.GetRepository())
	assert.NotEmpty(t, create.GetRepository().GetCloneHttpUrl())
	assert.NotEmpty(t, create.GetRepository().GetCloneSshUrl())
	assert.Equal(t, codespacev1.GitProtocol_GIT_PROTOCOL_SSH, create.GetRepository().GetPreferredProtocol())
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

func TestFetchOperationsSkipsCreateOutsideManagerUserScope(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	manager.UserID = 2
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("user_id").Update(manager)
	require.NoError(t, err)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE},
		AcceptedCreateTags:       []string{"default"},
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
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		StartupCapacityAvailable: 1,
		AcceptedOperationTypes: []codespacev1.AcceptedOperationType{
			codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE,
			codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME,
		},
		AcceptedCreateTags:       []string{"default"},
		CleanupCapacityAvailable: 1,
		ObservedOperations: []*codespacev1.ObservedOperation{
			{RuntimeUuid: runningCreateUUID, OperationRversion: 51},
			{RuntimeUuid: runningStopUUID, OperationRversion: 52},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 2)
	assert.NotNil(t, result.Operations[0].GetAbortCreate())
	assert.Zero(t, result.Operations[0].GetLeaseValidForMilliseconds())
	assert.NotNil(t, result.Operations[1].GetStop())
	require.Len(t, result.RenewedLeases, 1)
	assert.Equal(t, runningStopUUID, result.RenewedLeases[0].GetRuntimeUuid())

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
				assertServiceExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", tc.uuid)
			} else {
				assertServiceNotExists(t, new(codespace_model.GiteaToken), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", tc.uuid)
			}
			if tc.expectKey {
				assertServiceExists(t, new(codespace_model.SSHKey), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", tc.uuid)
			} else {
				assertServiceNotExists(t, new(codespace_model.SSHKey), "codespace_id = (SELECT id FROM codespace WHERE uuid = ?)", tc.uuid)
			}
		})
	}
}

func TestFetchOperationsRenewsObservedOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		ObservedOperations: []*codespacev1.ObservedOperation{{
			RuntimeUuid:       codespaceUUID,
			OperationRversion: 32,
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	require.Len(t, result.RenewedLeases, 1)
	assert.Equal(t, codespaceUUID, result.RenewedLeases[0].GetRuntimeUuid())
	assert.EqualValues(t, 32, result.RenewedLeases[0].GetOperationRversion())
	assert.EqualValues(t, setting.Codespace.OperationLeaseTimeout/time.Millisecond, result.RenewedLeases[0].GetLeaseValidForMilliseconds())
	assert.Greater(t, loadServiceCodespace(t, codespaceUUID).OperationDeadlineUnix, time.Now().Unix()+1)
}

func TestFetchOperationsRejectsStateHistoryConflictBeforeWrites(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		ObservedOperations: []*codespacev1.ObservedOperation{
			{RuntimeUuid: renewedUUID, OperationRversion: 36},
			{RuntimeUuid: conflictUUID, OperationRversion: 38},
		},
	})
	require.ErrorIs(t, err, ErrFetchStateHistoryConflict)
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, renewedUUID).OperationDeadlineUnix)
}

func TestFetchOperationsWaitsForUnobservedRunningOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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

	result, err := FetchOperations(t.Context(), manager, FetchOperationsOptions{})
	require.NoError(t, err)
	assert.Empty(t, result.Operations)
	assert.Empty(t, result.RenewedLeases)
	assert.Equal(t, originalDeadline, loadServiceCodespace(t, codespaceUUID).OperationDeadlineUnix)
}

func TestFetchOperationsReturnsCurrentPayloadForLowerObservedVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		ObservedOperations: []*codespacev1.ObservedOperation{{
			RuntimeUuid:       codespaceUUID,
			OperationRversion: 38,
		}},
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.Empty(t, result.RenewedLeases)
	assert.Equal(t, codespaceUUID, result.Operations[0].GetRuntimeUuid())
	assert.EqualValues(t, 39, result.Operations[0].GetOperationRversion())
	assert.NotNil(t, result.Operations[0].GetStop())
	assert.Greater(t, loadServiceCodespace(t, codespaceUUID).OperationDeadlineUnix, time.Now().Unix()+1)
}

func TestFetchOperationsClaimsCleanupStop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		CleanupCapacityAvailable: 1,
	})
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)
	assert.NotNil(t, result.Operations[0].GetStop())
	assert.Equal(t, codespaceUUID, result.Operations[0].GetRuntimeUuid())
	assert.Equal(t, codespace_model.OperationStatusRunning, loadServiceCodespace(t, codespaceUUID).OperationStatus)
}

func TestFetchOperationsRejectsStateHistoryConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
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
		ObservedOperations: []*codespacev1.ObservedOperation{{
			RuntimeUuid:       codespaceUUID,
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
