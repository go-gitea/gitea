// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strconv"
	"testing"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
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
	codespaceID := int64(1100)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 17,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 17, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "port-3000", Label: "App", Public: false},
	})))

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoID: 2})
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)

	row := result.Rows[0]
	assert.Equal(t, codespaceID, row.ID)
	assert.Equal(t, DisplayRunning, row.DisplayStatus)
	assert.Equal(t, DetailModeOverview, row.DetailMode)
	assert.Equal(t, "default", row.EnvironmentTag)
	assert.NotEmpty(t, row.CommitLink)
	assert.Equal(t, refreshStableMilliseconds, row.RefreshAfterMillis)
	require.NotNil(t, row.Workspace)
	assert.Equal(t, "/-/codespaces/"+strconv.FormatInt(row.ID, 10)+"/open", row.Workspace.OpenPath)
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

	ids := []int64{
		int64(1052),
		int64(1101),
		int64(1102),
	}
	environmentTags := []string{"rust", "python", "go"}
	for index, id := range ids {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{ID: id, Status: codespace_model.StatusStopped})
		_, err := db.GetEngine(t.Context()).Where("id = ?", id).Cols("repo_id", "environment_tag", "updated_unix").Update(&codespace_model.Codespace{
			RepoID:         3,
			EnvironmentTag: environmentTags[index],
			UpdatedUnix:    int64(index + 2),
		})
		require.NoError(t, err)
	}
	_, err := db.GetEngine(t.Context()).Where("id = ?", ids[2]).Cols("user_id").Update(&codespace_model.Codespace{UserID: 2})
	require.NoError(t, err)

	first, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoOwnerID: 3, Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, first.Total)
	require.Len(t, first.Rows, 1)
	assert.Equal(t, ids[1], first.Rows[0].ID)

	second, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoOwnerID: 3, Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, second.Total)
	require.Len(t, second.Rows, 1)
	assert.Equal(t, ids[0], second.Rows[0].ID)

	byRepository, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, Keyword: "ORG3/REPO3", Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, byRepository.Total)
	require.Len(t, byRepository.Rows, 1)

	byEnvironment, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, Keyword: "python", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 1, byEnvironment.Total)
	require.Len(t, byEnvironment.Rows, 1)
	assert.Equal(t, ids[1], byEnvironment.Rows[0].ID)
}

func TestListCreatorCodespacesAppliesLimit(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, id := range []int64{
		int64(1050),
		int64(1103),
		int64(1104),
	} {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{
			ID:     id,
			Status: codespace_model.StatusStopped,
		})
	}

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoID: 2, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, result.Rows, 2)
}

func TestListCreatorCodespacesFiltersExactSource(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pullID := int64(1051)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{ID: pullID, Status: codespace_model.StatusStopped})
	_, err := db.GetEngine(t.Context()).Where("id = ?", pullID).Cols("ref_type", "ref_name").Update(&codespace_model.Codespace{
		RefType: "pull",
		RefName: "refs/pull/3/head",
	})
	require.NoError(t, err)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{ID: int64(1105), Status: codespace_model.StatusStopped})

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
	matchingIDs := []int64{
		int64(1000),
		int64(1032),
	}
	refTypes := []string{"branch", "tag"}
	refNames := []string{"main", "v1.0.0"}
	for index, id := range matchingIDs {
		insertServiceCodespace(t, 0, &codespace_model.Codespace{ID: id, Status: codespace_model.StatusRunning})
		_, err := db.GetEngine(t.Context()).Where("id = ?", id).Cols("ref_type", "ref_name", "commit_sha").Update(&codespace_model.Codespace{
			RefType:   refTypes[index],
			RefName:   refNames[index],
			CommitSHA: commitSHA,
		})
		require.NoError(t, err)
	}
	nonMatchingID := int64(1106)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{ID: nonMatchingID, Status: codespace_model.StatusRunning})
	_, err := db.GetEngine(t.Context()).Where("id = ?", nonMatchingID).Cols("commit_sha").Update(&codespace_model.Codespace{
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
	assert.ElementsMatch(t, matchingIDs, []int64{result.Rows[0].ID, result.Rows[1].ID})
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
	_, err := db.GetEngine(t.Context()).ID(manager.ID).Cols("gateway_url", "gateway_ssh_addr", "gateway_ssh_host_key_algorithm", "gateway_ssh_host_key_fingerprint_sha256").Update(&codespace_model.Manager{
		GatewayURL:                         "https://gateway.example.com",
		GatewaySSHAddr:                     "ssh.example.com:2222",
		GatewaySSHHostKeyAlgorithm:         "ssh-ed25519",
		GatewaySSHHostKeyFingerprintSHA256: "SHA256:view",
	})
	require.NoError(t, err)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1107)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                   codespaceID,
		Status:               codespace_model.StatusRunning,
		OperationRVersion:    18,
		OperationTrigger:     codespace_model.OperationTriggerIdle,
		OperationCreatedUnix: time.Now().Unix(),
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, serviceRuntimeMetadataEntry(t, 18, []*codespacev1.RuntimeEndpoint{
		{EndpointId: "private-app", Label: "Private app", Public: false, Port: 3000},
		{EndpointId: "public-app", Label: "Public app", Public: true, Port: 8080},
	})))

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: codespaceID})
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
	assert.EqualValues(t, 3000, view.Endpoints[0].Port)
	assert.EqualValues(t, 8080, view.Endpoints[1].Port)
	assert.Equal(t, "https://private-app.gateway.example.com/", view.Endpoints[0].URL)
	require.NotNil(t, view.SSH)
	assert.Equal(t, "ssh -p 2222 reported-runtime@ssh.example.com", view.SSH.Command)
	assert.Equal(t, "vscode://vscode-remote/ssh-remote+reported-runtime@ssh.example.com:2222/workspaces/project", string(view.SSH.VSCodeURL))
	assert.Equal(t, "zed://ssh/reported-runtime@ssh.example.com:2222/workspaces/project", string(view.SSH.ZedURL))
	assert.Equal(t, "ssh-ed25519", view.SSH.HostKeyAlgorithm)
	assert.Equal(t, "SHA256:view", view.SSH.HostKeyFingerprint)
}

func TestCreatorSSHViewFormatsConnection(t *testing.T) {
	for _, tc := range []struct {
		uri     string
		command string
		remote  string
	}{
		{"ssh://runtime@ssh.example.com:22", "ssh runtime@ssh.example.com", "runtime@ssh.example.com"},
		{"ssh://runtime@[2001:db8::1]:2222", "ssh -p 2222 runtime@[2001:db8::1]", "runtime@%5B2001:db8::1%5D:2222"},
		{"ssh://runtime@[2001:db8::1]:22", "ssh runtime@[2001:db8::1]", "runtime@%5B2001:db8::1%5D"},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			view := creatorSSHView(&codespace_model.Manager{}, runtimeMetadata{
				WorkspaceFolder: "/workspaces/项目 #1",
				SSH:             tc.uri,
			})
			require.NotNil(t, view)
			assert.Equal(t, tc.command, view.Command)
			assert.Equal(t, "vscode://vscode-remote/ssh-remote+"+tc.remote+"/workspaces/%E9%A1%B9%E7%9B%AE%20%231", string(view.VSCodeURL))
			assert.Equal(t, "zed://ssh/"+tc.remote+"/workspaces/%E9%A1%B9%E7%9B%AE%20%231", string(view.ZedURL))
		})
	}
}

func TestGetCreatorCodespaceRunsWithoutWorkspaceEndpoint(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1108)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 46,
	})
	entry := serviceRuntimeMetadataEntry(t, 46, nil)
	entry.Metadata.Endpoints = nil
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, entry))

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: codespaceID})
	require.NoError(t, err)
	assert.Equal(t, DisplayRunning, view.DisplayStatus)
	assert.Nil(t, view.Workspace)
	assert.False(t, view.CanOpen)
}

func TestGetCreatorCodespaceShowsTransitionsAndPermissions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	codespaceID := int64(1109)
	insertServiceCodespace(t, 0, &codespace_model.Codespace{
		ID:                   codespaceID,
		Status:               codespace_model.StatusCreating,
		OperationTrigger:     codespace_model.OperationTriggerUser,
		OperationCreatedUnix: 1,
	})

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: codespaceID})
	require.NoError(t, err)
	assert.Equal(t, DisplayQueued, view.DisplayStatus)
	assert.Equal(t, DetailModeLogs, view.DetailMode)
	assert.Equal(t, refreshTransitionMilliseconds, view.RefreshAfterMillis)
	assert.False(t, view.CanOpen)
	assert.True(t, view.CanDelete)

	_, err = GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 2, CodespaceID: codespaceID})
	require.ErrorIs(t, err, ErrViewNotFound)
}

func TestGetCreatorCodespaceShowsCurrentBootStage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	codespaceID := int64(1110)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:                codespaceID,
		Status:            codespace_model.StatusCreating,
		OperationRVersion: 29,
	})
	entry := serviceRuntimeMetadataEntry(t, 29, nil)
	entry.Metadata.Boot.Stage = codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_WORKSPACE
	require.NoError(t, putRuntimeMetadataEntry(codespaceID, entry))

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: codespaceID})
	require.NoError(t, err)
	assert.Equal(t, DisplayBooting, view.DisplayStatus)
	assert.Equal(t, DetailModeLogs, view.DetailMode)
	assert.Equal(t, "codespace.boot_stage.prepare_workspace", view.BootStageKey)
}

func TestStoppedCreatorCodespaceResumeRequiresOnlineManager(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	offlineID := int64(1025)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:     offlineID,
		Status: codespace_model.StatusStopped,
	})

	offlineView, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: offlineID})
	require.NoError(t, err)
	assert.Equal(t, DisplayStopped, offlineView.DisplayStatus)
	assert.False(t, offlineView.CanResume)
	assert.Equal(t, "codespace.stopped_manager_unavailable", offlineView.StatusSummaryKey)

	markServiceManagerOnline(t, manager, `[{"tag":"default"}]`)
	onlineID := int64(1027)
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		ID:     onlineID,
		Status: codespace_model.StatusStopped,
	})

	onlineView, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: onlineID})
	require.NoError(t, err)
	assert.Equal(t, DisplayStopped, onlineView.DisplayStatus)
	assert.True(t, onlineView.CanResume)
	assert.Equal(t, "codespace.status_summary.stopped", onlineView.StatusSummaryKey)

	_, err = db.GetEngine(t.Context()).ID(manager.ID).Cols("runtime_state").Update(&codespace_model.Manager{
		RuntimeState: codespace_model.ManagerRuntimeStateRecovering,
	})
	require.NoError(t, err)
	recoveringView, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceID: onlineID})
	require.NoError(t, err)
	assert.False(t, recoveringView.CanResume)
}
