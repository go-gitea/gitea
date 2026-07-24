// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCreatorCodespacesShowsRunningReadyEndpoints(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
	codespaceUUID := "17171717-1717-4717-8717-171717171717"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:              codespaceUUID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 17,
	})
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 17, []map[string]any{
		{"endpoint_id": "app-3000", "label": "App", "public": false},
	})))

	result, err := ListCreatorCodespaces(t.Context(), CreatorListOptions{UserID: 1, RepoID: 2})
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)

	row := result.Rows[0]
	assert.Equal(t, codespaceUUID, row.UUID)
	assert.Equal(t, DisplayRunning, row.DisplayStatus)
	assert.Equal(t, refreshStableMilliseconds, row.RefreshAfterMillis)
	require.NotNil(t, row.Workspace)
	assert.Equal(t, "/-/codespaces/"+codespaceUUID+"/open", row.Workspace.OpenPath)
	require.Len(t, row.Endpoints, 1)
	assert.Equal(t, "app-3000", row.Endpoints[0].EndpointID)
	assert.True(t, row.CanOpen)
	assert.True(t, row.CanStop)
	assert.True(t, row.CanDelete)
	assert.True(t, row.CanConfigureAutoStop)
}

func TestGetCreatorCodespaceKeepsQueuedIdleStopInteractive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	markServiceManagerOnline(t, manager, `["default"]`)
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
	require.NoError(t, putRuntimeMetadataEntry(codespaceUUID, serviceRuntimeMetadataEntry(t, 18, []map[string]any{})))

	view, err := GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 1, CodespaceUUID: codespaceUUID})
	require.NoError(t, err)

	assert.Equal(t, DisplayRunning, view.DisplayStatus)
	assert.Equal(t, refreshStableMilliseconds, view.RefreshAfterMillis)
	assert.True(t, view.CanOpen)
	assert.True(t, view.CanContinue)
	assert.True(t, view.CanStop)
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
	assert.Equal(t, refreshTransitionMilliseconds, view.RefreshAfterMillis)
	assert.False(t, view.CanOpen)
	assert.True(t, view.CanDelete)

	_, err = GetCreatorCodespace(t.Context(), CreatorDetailOptions{UserID: 2, CodespaceUUID: codespaceUUID})
	require.ErrorIs(t, err, ErrViewPermissionDenied)
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

	markServiceManagerOnline(t, manager, `["default"]`)
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
