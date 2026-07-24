// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestUUIDValidation(t *testing.T) {
	generated := NewUUID()
	require.NoError(t, ValidateUUID(generated))

	uuid32, err := UUID32(generated)
	require.NoError(t, err)
	assert.Len(t, uuid32, 32)

	assert.Error(t, ValidateUUID("11111111-1111-1111-8111-111111111111"))
	assert.Error(t, ValidateUUID("11111111111141118111111111111111"))
	assert.Error(t, ValidateUUID("A0EEBC99-9C0B-4EF8-BB6D-6BB9BD380A11"))
}

func TestNextVersion(t *testing.T) {
	next, err := NextVersion(0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, next)

	next, err = NextVersion(41)
	require.NoError(t, err)
	assert.EqualValues(t, 42, next)

	assert.Error(t, func() error {
		_, err := NextVersion(-1)
		return err
	}())
	assert.Error(t, func() error {
		_, err := NextVersion(math.MaxInt64)
		return err
	}())
}

func TestManagerSecretVerifier(t *testing.T) {
	manager := &Manager{}
	secret := manager.GenerateManagerSecret()
	assert.Len(t, secret, 64)
	assert.Len(t, manager.SecretSalt, 32)
	assert.Len(t, manager.SecretHash, 64)
	assert.True(t, manager.VerifyManagerSecret(secret))
	assert.False(t, manager.VerifyManagerSecret("bad-secret"))
}

func TestCodespaceTableIndices(t *testing.T) {
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "user_status", "user_id", "status")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "repo_status", "repo_id", "status")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "create_claim", "status", "operation_type", "operation_status", "manager_id", "repo_tag", "operation_created_unix", "uuid")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "manager_active", "manager_id", "operation_type", "operation_status", "status", "operation_created_unix", "uuid")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "queued_timeout", "operation_status", "operation_created_unix", "uuid")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "running_timeout", "operation_status", "operation_deadline_unix", "uuid")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "failed_retention", "status", "updated_unix", "uuid")
}

func TestManagerTableIndices(t *testing.T) {
	assertIndexColumns(t, (&Manager{}).TableIndices(), "owner_runtime", "owner_id", "runtime_state")
	assertIndexColumns(t, (&Manager{}).TableIndices(), "runtime_online", "runtime_state", "last_online_unix")
}

func TestValidateCodespace(t *testing.T) {
	for _, status := range []string{StatusCreating, StatusRunning, StatusStopped, StatusDeleting, StatusFailed} {
		t.Run("status/"+status, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.Status = status
			require.NoError(t, ValidateCodespace(row))
		})
	}

	for _, operationType := range []string{OperationCreate, OperationResume, OperationStop, OperationDelete} {
		t.Run("operation type/"+operationType, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.OperationType = operationType
			row.OperationStatus = OperationStatusQueued
			row.OperationTrigger = OperationTriggerUser
			require.NoError(t, ValidateCodespace(row))
		})
	}

	for _, trigger := range []string{OperationTriggerUser, OperationTriggerIdle} {
		t.Run("operation trigger/"+trigger, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.OperationType = OperationCreate
			row.OperationStatus = OperationStatusQueued
			row.OperationTrigger = trigger
			require.NoError(t, ValidateCodespace(row))
		})
	}

	for _, protocol := range []string{GitProtocolHTTP, GitProtocolSSH} {
		t.Run("git protocol/"+protocol, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.GitProtocol = protocol
			require.NoError(t, ValidateCodespace(row))
		})
	}

	for _, mode := range []string{AutoStopModeDefault, AutoStopModeCustom, AutoStopModeNever} {
		t.Run("auto stop mode/"+mode, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.AutoStopMode = mode
			require.NoError(t, ValidateCodespace(row))
		})
	}

	row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.Status = "booting"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.OperationType = OperationCreate
	row.OperationStatus = OperationStatusQueued
	row.OperationTrigger = OperationTriggerUser
	require.NoError(t, ValidateCodespace(row))

	row.OperationStatus = "leased"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.OperationType = "snapshot"
	row.OperationStatus = OperationStatusQueued
	row.OperationTrigger = OperationTriggerUser
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.OperationType = OperationCreate
	row.OperationStatus = OperationStatusQueued
	row.OperationTrigger = "timer"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.GitProtocol = "git"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.AutoStopMode = "disabled"
	assert.Error(t, ValidateCodespace(row))
}

func TestValidateManager(t *testing.T) {
	for _, state := range []string{ManagerRuntimeStateOnline, ManagerRuntimeStateRecovering} {
		t.Run(state, func(t *testing.T) {
			require.NoError(t, ValidateManager(&Manager{RuntimeState: state}))
		})
	}

	assert.Error(t, ValidateManager(nil))
	assert.Error(t, ValidateManager(&Manager{RuntimeState: ""}))
	assert.Error(t, ValidateManager(&Manager{RuntimeState: "offline"}))
}

func assertIndexColumns(t *testing.T, indexes []*schemas.Index, name string, columns ...string) {
	t.Helper()
	for _, index := range indexes {
		if index.Name == name {
			assert.Equal(t, schemas.IndexType, index.Type)
			assert.Equal(t, columns, index.Cols)
			return
		}
	}
	assert.Failf(t, "missing index", "index %q was not declared", name)
}

func validCodespace(codespaceUUID string) *Codespace {
	return &Codespace{
		UUID:           codespaceUUID,
		UserID:         1,
		RepoID:         2,
		RefType:        "branch",
		RefName:        "main",
		RepoTag:        "default",
		GitProtocol:    GitProtocolHTTP,
		CommitSHA:      "0123456789abcdef0123456789abcdef01234567",
		Status:         StatusCreating,
		AutoStopMode:   AutoStopModeDefault,
		CreatedUnix:    1,
		UpdatedUnix:    1,
		LogFilename:    "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb.log",
		LogLineCount:   0,
		LogSize:        0,
		StoppedUnix:    0,
		LastActiveUnix: 0,
	}
}
