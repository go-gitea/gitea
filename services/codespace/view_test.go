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

func TestListCreatorCodespacesShowsRunningWorkspace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "17171717-1717-4717-8717-171717171717"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 17,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 17, []map[string]any{
		{"endpoint_id": "port-3000", "label": "App", "public": false},
	})))

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoID: 2})
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)

	row := result.Rows[0]
	assert.Equal(t, codespaceUUID, row.UUID)
	assert.Equal(t, DisplayRunning, row.DisplayStatus)
	assert.Equal(t, DetailModeOverview, row.DetailMode)
	assert.Equal(t, "default", row.EnvironmentTag)
	assert.NotEmpty(t, row.CommitLink)
	assert.Equal(t, refreshStableMilliseconds, row.RefreshAfterMillis)
	require.NotNil(t, row.Workspace)
	assert.Equal(t, "/-/codespaces/"+codespaceUUID+"/open", row.Workspace.OpenPath)
	assert.Empty(t, row.Endpoints)
	assert.Nil(t, row.ResourceUsage)
	assert.Nil(t, row.SSH)
	assert.True(t, row.CanOpen)
	assert.True(t, row.CanStop)
	assert.True(t, row.CanDelete)
	assert.True(t, row.CanConfigureAutoStop)
}

func TestListCreatorCodespacesPaginatesWithinRepositoryOwner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	uuids := []string{
		"36363636-3636-4636-8636-363636363636",
		"37373737-3737-4737-8737-373737373737",
		"38383838-3838-4838-8838-383838383838",
	}
	for index, uuid := range uuids {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{UUID: uuid, Status: codespace_model.StatusStopped})
		_, err := db.GetEngine(t.Context()).ID(uuid).Cols("repo_id", "updated_unix").Update(&codespace_model.Codespace{
			RepoID:      3,
			UpdatedUnix: int64(index + 2),
		})
		require.NoError(t, err)
	}
	_, err := db.GetEngine(t.Context()).ID(uuids[2]).Cols("user_id").Update(&codespace_model.Codespace{UserID: 2})
	require.NoError(t, err)

	first, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoOwnerID: 3, Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, first.Total)
	require.Len(t, first.Rows, 1)
	assert.Equal(t, uuids[1], first.Rows[0].UUID)

	second, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoOwnerID: 3, Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, second.Total)
	require.Len(t, second.Rows, 1)
	assert.Equal(t, uuids[0], second.Rows[0].UUID)
}

func TestListCreatorCodespacesAppliesLimit(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, uuid := range []string{
		"31313131-3131-4131-8131-313131313131",
		"32323232-3232-4232-8232-323232323232",
		"33333333-3333-4333-8333-333333333333",
	} {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{
			UUID:   uuid,
			Status: codespace_model.StatusStopped,
		})
	}

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoID: 2, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, result.Rows, 2)
}

func TestListCreatorCodespacesFiltersExactSource(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pullUUID := "34343434-3434-4434-8434-343434343434"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{UUID: pullUUID, Status: codespace_model.StatusStopped})
	_, err := db.GetEngine(t.Context()).ID(pullUUID).Cols("ref_type", "ref_name").Update(&codespace_model.Codespace{
		RefType: "pull",
		RefName: "refs/pull/3/head",
	})
	require.NoError(t, err)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{UUID: "35353535-3535-4535-8535-353535353535", Status: codespace_model.StatusStopped})

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{
		UserID:  1,
		RepoID:  2,
		RefType: "pull",
		RefName: "refs/pull/3/head",
	})
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "#3", result.Rows[0].RefDisplayName)
}

func TestListCreatorCodespacesFiltersCommitSHA(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	commitSHA := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	matchingUUIDs := []string{
		"39393939-3939-4939-8939-393939393939",
		"40404040-4040-4040-8040-404040404040",
	}
	refTypes := []string{"branch", "tag"}
	refNames := []string{"main", "v1.0.0"}
	for index, uuid := range matchingUUIDs {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{UUID: uuid, Status: codespace_model.StatusRunning})
		_, err := db.GetEngine(t.Context()).ID(uuid).Cols("ref_type", "ref_name", "commit_sha").Update(&codespace_model.Codespace{
			RefType:   refTypes[index],
			RefName:   refNames[index],
			CommitSHA: commitSHA,
		})
		require.NoError(t, err)
	}
	nonMatchingUUID := "41414141-4141-4141-8141-414141414141"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{UUID: nonMatchingUUID, Status: codespace_model.StatusRunning})
	_, err := db.GetEngine(t.Context()).ID(nonMatchingUUID).Cols("commit_sha").Update(&codespace_model.Codespace{
		CommitSHA: "4a357436d925b5c974181ff12a994538ddc5a269",
	})
	require.NoError(t, err)

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{
		UserID:    1,
		RepoID:    2,
		CommitSHA: commitSHA,
	})
	require.NoError(t, err)
	require.Len(t, result.Rows, 2)
	assert.ElementsMatch(t, matchingUUIDs, []string{result.Rows[0].UUID, result.Rows[1].UUID})
}

func TestCreatorAutoStopViewUsesExactHumanUnits(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopDefaultTimeout, 30*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopMinTimeout, 5*time.Minute))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.AutoStopMaxTimeout, 7*24*time.Hour))

	view := creatorAutoStopView(&codespace_model.Codespace{
		AutoStopMode:           codespace_model.AutoStopModeCustom,
		AutoStopTimeoutSeconds: 90 * 60,
	})

	assert.Equal(t, CreatorDurationView{Value: 90, Unit: "minutes", TranslationKey: "tool.minutes"}, view.Timeout)
	assert.Equal(t, CreatorDurationView{Value: 30, Unit: "minutes", TranslationKey: "tool.minutes"}, view.Default)
	assert.Equal(t, CreatorDurationView{Value: 5, Unit: "minutes", TranslationKey: "tool.minutes"}, view.Minimum)
	assert.Equal(t, CreatorDurationView{Value: 7, Unit: "days", TranslationKey: "tool.days"}, view.Maximum)
	assert.True(t, view.EffectiveEnabled)
	assert.Equal(t, view.Timeout, view.EffectiveTimeout)
	assert.False(t, view.CustomTimeoutOutOfRange)

	view = creatorAutoStopView(&codespace_model.Codespace{
		AutoStopMode:           codespace_model.AutoStopModeCustom,
		AutoStopTimeoutSeconds: 301,
	})
	assert.Equal(t, CreatorDurationView{Value: 301, Unit: "seconds", TranslationKey: "tool.seconds"}, view.Timeout)
	assert.False(t, view.CustomTimeoutOutOfRange)

	view = creatorAutoStopView(&codespace_model.Codespace{AutoStopMode: codespace_model.AutoStopModeDefault})
	assert.Equal(t, view.Default, view.Timeout)
	assert.True(t, view.EffectiveEnabled)

	view = creatorAutoStopView(&codespace_model.Codespace{AutoStopMode: codespace_model.AutoStopModeNever})
	assert.Equal(t, view.Default, view.Timeout)
	assert.False(t, view.EffectiveEnabled)

	view = creatorAutoStopView(&codespace_model.Codespace{
		AutoStopMode:           codespace_model.AutoStopModeCustom,
		AutoStopTimeoutSeconds: 4 * 60,
	})
	assert.True(t, view.CustomTimeoutOutOfRange)
}

func TestGetCreatorCodespaceKeepsQueuedIdleStopInteractive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("gateway_ssh_host_key_algorithm", "gateway_ssh_host_key_fingerprint_sha256", "gateway_ssh_host_key_updated_unix").Update(&codespace_model.Manager{
		GatewaySSHHostKeyAlgorithm:         "ssh-ed25519",
		GatewaySSHHostKeyFingerprintSHA256: "SHA256:view",
		GatewaySSHHostKeyUpdatedUnix:       123,
	})
	require.NoError(t, err)
	insertSettingsManagerAddress(t, manager.ID, codespace_model.ManagerAddressSSH, "ssh.example.com:2222")
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "18181818-1818-4818-8818-181818181818"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                 codespaceUUID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    18,
		OperationType:        codespace_model.OperationStop,
		OperationStatus:      codespace_model.OperationStatusQueued,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 18, []map[string]any{
		{"endpoint_id": "private-app", "label": "Private app", "public": false},
		{"endpoint_id": "public-app", "label": "Public app", "public": true},
	})))

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)

	assert.Equal(t, DisplayRunning, view.DisplayStatus)
	assert.Equal(t, DetailModeOverview, view.DetailMode)
	assert.Equal(t, refreshStableMilliseconds, view.RefreshAfterMillis)
	assert.True(t, view.CanOpen)
	assert.True(t, view.CanContinue)
	assert.True(t, view.CanStop)
	require.Len(t, view.Endpoints, 2)
	assert.True(t, view.Endpoints[0].CanOpen)
	assert.False(t, view.Endpoints[1].CanOpen)
	require.NotNil(t, view.SSH)
	assert.Equal(t, "ssh -p 2222 cs-"+codespaceUUID+"@ssh.example.com", view.SSH.Command)
	assert.Equal(t, "ssh-ed25519", view.SSH.HostKeyAlgorithm)
	assert.Equal(t, "SHA256:view", view.SSH.HostKeyFingerprint)
	assert.EqualValues(t, 123, view.SSH.HostKeyUpdatedUnix)
}

func TestGetCreatorCodespaceShowsTransitionsAndPermissions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceUUID := "19191919-1919-4919-8919-191919191919"
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		UUID:            codespaceUUID,
		Status:          codespace_model.StatusCreating,
		OperationType:   codespace_model.OperationCreate,
		OperationStatus: codespace_model.OperationStatusQueued,
	})

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.Equal(t, DisplayQueued, view.DisplayStatus)
	assert.Equal(t, DetailModeLogs, view.DetailMode)
	assert.Equal(t, refreshTransitionMilliseconds, view.RefreshAfterMillis)
	assert.False(t, view.CanOpen)
	assert.True(t, view.CanDelete)

	_, err = GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 2, CodespaceUUID: codespaceUUID})
	require.ErrorIs(t, err, ErrViewPermissionDenied)
}

func TestGetCreatorCodespaceShowsCurrentBootStage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceUUID := "29292929-2929-4929-8929-292929292929"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusCreating,
		OperationType:     codespace_model.OperationCreate,
		OperationStatus:   codespace_model.OperationStatusRunning,
		OperationRVersion: 29,
	})
	entry := serviceRuntimeMetadataEntry(t, 29, nil)
	entry.Metadata.Boot.Stage = bootStagePrepareWorkspace
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, entry))

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)
	assert.Equal(t, DisplayBooting, view.DisplayStatus)
	assert.Equal(t, DetailModeLogs, view.DetailMode)
	assert.Equal(t, "codespace.boot_stage.prepare_workspace", view.BootStageKey)
}

func TestStoppedCreatorCodespaceResumeRequiresOnlineManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	offlineUUID := "20202020-2020-4020-8020-202020202020"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:   offlineUUID,
		Status: codespace_model.StatusStopped,
	})

	offlineView, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: offlineUUID})
	require.NoError(t, err)
	assert.Equal(t, DisplayStopped, offlineView.DisplayStatus)
	assert.False(t, offlineView.CanResume)

	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	onlineUUID := "21212121-2121-4121-8121-212121212121"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:   onlineUUID,
		Status: codespace_model.StatusStopped,
	})

	onlineView, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: onlineUUID})
	require.NoError(t, err)
	assert.Equal(t, DisplayStopped, onlineView.DisplayStatus)
	assert.True(t, onlineView.CanResume)

	_, err = db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(&codespace_model.Manager{
		RuntimeState: codespace_model.ManagerRuntimeStateRecovering,
	})
	require.NoError(t, err)
	recoveringView, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: onlineUUID})
	require.NoError(t, err)
	assert.False(t, recoveringView.CanResume)
}
