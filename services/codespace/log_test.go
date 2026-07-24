// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"io"
	"strings"
	"testing"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/dbfs"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateLogAppendsAndReplaysIdempotently(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "67676767-6767-4676-8676-676767676767"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     22,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	lines := []LogLine{
		{TimestampUnixNano: time.Date(2026, 7, 22, 1, 2, 3, 4, time.UTC).UnixNano(), Message: "prepare workspace"},
		{TimestampUnixNano: time.Date(2026, 7, 22, 1, 2, 4, 5, time.UTC).UnixNano(), Message: "ready"},
	}

	result, err := UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 22,
		Offset:            0,
		Lines:             lines,
	})
	require.NoError(t, err)
	require.Positive(t, result.NextOffset)
	row := loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, result.NextOffset, row.LogSize)
	assert.EqualValues(t, 2, row.LogLineCount)
	content := readServiceLog(t, row.LogFilename)
	assert.Contains(t, content, "prepare workspace\n")
	assert.Contains(t, content, "ready\n")

	beforeReplay := readServiceLog(t, row.LogFilename)
	replay, err := UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 22,
		Offset:            0,
		Lines:             lines,
	})
	require.NoError(t, err)
	assert.Equal(t, result.NextOffset, replay.NextOffset)
	assert.Equal(t, beforeReplay, readServiceLog(t, row.LogFilename))
}

func TestUpdateLogRejectsOffsetGapAndConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "78787878-7878-4787-8787-787878787878"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     23,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	first, err := UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 23,
		Offset:            0,
		Lines: []LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 1, 3, 0, 0, time.UTC).UnixNano(),
			Message:           "first",
		}},
	})
	require.NoError(t, err)

	_, err = UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 23,
		Offset:            first.NextOffset + 1,
		Lines: []LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 1, 3, 1, 0, time.UTC).UnixNano(),
			Message:           "gap",
		}},
	})
	var offsetErr *LogOffsetError
	require.ErrorAs(t, err, &offsetErr)
	assert.ErrorIs(t, err, ErrUpdateLogOffsetGap)
	assert.Equal(t, first.NextOffset, offsetErr.CurrentOffset)

	_, err = UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 23,
		Offset:            0,
		Lines: []LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 1, 3, 0, 0, time.UTC).UnixNano(),
			Message:           "different",
		}},
	})
	require.ErrorAs(t, err, &offsetErr)
	assert.ErrorIs(t, err, ErrUpdateLogOffsetConflict)
	assert.Equal(t, first.NextOffset, offsetErr.CurrentOffset)
}

func TestUpdateLogRejectsStaleOperation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	otherManager := insertServiceManager(t)
	codespaceUUID := "89898989-8989-4898-8989-898989898989"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     24,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})

	_, err := UpdateLog(t.Context(), otherManager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 24,
		Offset:            0,
		Lines: []LogLine{{
			TimestampUnixNano: time.Now().UnixNano(),
			Message:           "stale",
		}},
	})
	require.ErrorIs(t, err, ErrUpdateLogStaleOperation)
	assert.Zero(t, loadServiceCodespace(t, codespaceUUID).LogSize)
}

func TestUpdateLogAppendsTruncationSummaryOnce(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.Codespace.LogMaxSize, codespaceLogFinalSummaryReserve+260))

	manager := insertServiceManager(t)
	codespaceUUID := "93909090-9090-4090-8090-909090909090"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     27,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	first, err := UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 27,
		Offset:            0,
		Lines: []LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 1, 4, 0, 0, time.UTC).UnixNano(),
			Message:           strings.Repeat("a", 100),
		}},
	})
	require.NoError(t, err)
	assert.Less(t, first.NextOffset, codespaceLogOrdinaryLimit())

	_, err = UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 27,
		Offset:            first.NextOffset,
		Lines: []LogLine{
			{
				TimestampUnixNano: time.Date(2026, 7, 22, 1, 4, 1, 0, time.UTC).UnixNano(),
				Message:           strings.Repeat("b", 100),
			},
			{
				TimestampUnixNano: time.Date(2026, 7, 22, 1, 4, 2, 0, time.UTC).UnixNano(),
				Message:           strings.Repeat("c", 100),
			},
		},
	})
	require.ErrorIs(t, err, ErrUpdateLogSizeExceeded)
	row := loadServiceCodespace(t, codespaceUUID)
	require.LessOrEqual(t, row.LogSize, setting.Codespace.LogMaxSize)
	content := readServiceLog(t, row.LogFilename)
	assert.NotContains(t, content, strings.Repeat("b", 100))
	assert.NotContains(t, content, strings.Repeat("c", 100))
	assert.Equal(t, 1, strings.Count(content, codespaceLogTruncationMessage))
	assert.EqualValues(t, 2, row.LogLineCount)

	sizeAfterSummary := row.LogSize
	_, err = UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 27,
		Offset:            row.LogSize,
		Lines: []LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 1, 4, 3, 0, time.UTC).UnixNano(),
			Message:           "after limit",
		}},
	})
	require.ErrorIs(t, err, ErrUpdateLogSizeExceeded)
	row = loadServiceCodespace(t, codespaceUUID)
	assert.Equal(t, sizeAfterSummary, row.LogSize)
	assert.Equal(t, content, readServiceLog(t, row.LogFilename))
}

func TestReadLogPagesByReturnedOffset(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "90909090-9090-4090-8090-909090909090"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     25,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	result, err := UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 25,
		Offset:            0,
		Lines: []LogLine{
			{TimestampUnixNano: time.Date(2026, 7, 22, 2, 0, 0, 0, time.UTC).UnixNano(), Message: "first"},
			{TimestampUnixNano: time.Date(2026, 7, 22, 2, 0, 1, 0, time.UTC).UnixNano(), Message: "second"},
			{TimestampUnixNano: time.Date(2026, 7, 22, 2, 0, 2, 0, time.UTC).UnixNano(), Message: "third"},
		},
	})
	require.NoError(t, err)

	firstPage, err := ReadLog(t.Context(), ReadLogOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		Offset:        0,
		Limit:         1,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 0, firstPage.Offset)
	assert.False(t, firstPage.EOF)
	assert.True(t, firstPage.Truncated)
	require.Len(t, firstPage.Lines, 1)
	assert.Contains(t, firstPage.Lines[0], "first\n")

	secondPage, err := ReadLog(t.Context(), ReadLogOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		Offset:        firstPage.NextOffset,
		Limit:         1,
	})
	require.NoError(t, err)
	assert.False(t, secondPage.EOF)
	assert.True(t, secondPage.Truncated)
	require.Len(t, secondPage.Lines, 1)
	assert.Contains(t, secondPage.Lines[0], "second\n")

	eofPage, err := ReadLog(t.Context(), ReadLogOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		Offset:        result.NextOffset,
		Limit:         LogReadMaxBytes,
	})
	require.NoError(t, err)
	assert.True(t, eofPage.EOF)
	assert.Empty(t, eofPage.Lines)
}

func TestReadLogRejectsOffsetPastEOF(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	manager := insertServiceManager(t)
	codespaceUUID := "91909090-9090-4090-8090-909090909090"
	insertServiceCodespace(t, manager.ID, &codespace_model.Codespace{
		UUID:                  codespaceUUID,
		Status:                codespace_model.StatusCreating,
		OperationRVersion:     26,
		OperationType:         codespace_model.OperationCreate,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationTrigger:      codespace_model.OperationTriggerUser,
		OperationCreatedUnix:  10,
		OperationStartedUnix:  11,
		OperationDeadlineUnix: time.Now().Add(time.Hour).Unix(),
	})
	result, err := UpdateLog(t.Context(), manager, UpdateLogOptions{
		CodespaceUUID:     codespaceUUID,
		OperationRVersion: 26,
		Offset:            0,
		Lines: []LogLine{{
			TimestampUnixNano: time.Date(2026, 7, 22, 2, 1, 0, 0, time.UTC).UnixNano(),
			Message:           "line",
		}},
	})
	require.NoError(t, err)

	_, err = ReadLog(t.Context(), ReadLogOptions{
		UserID:        1,
		CodespaceUUID: codespaceUUID,
		Offset:        result.NextOffset + 1,
		Limit:         LogReadMaxBytes,
	})
	var offsetErr *LogOffsetError
	require.ErrorAs(t, err, &offsetErr)
	assert.ErrorIs(t, err, ErrReadLogOffsetConflict)
	assert.Equal(t, result.NextOffset, offsetErr.CurrentOffset)
}

func readServiceLog(t *testing.T, filename string) string {
	t.Helper()
	file, err := dbfs.Open(t.Context(), codespaceLogDBFSPrefix+filename)
	require.NoError(t, err)
	defer file.Close()
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(data)
}
