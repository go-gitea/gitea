// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestUUIDValidation(t *testing.T) {
	generated := uuid.NewString()
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
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "user_updated", "user_id", "updated_unix", "created_unix", "id")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "repo", "repo_id")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "create_claim", "operation_started_unix", "status", "manager_id", "environment_tag", "operation_created_unix", "id")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "manager_active", "manager_id", "operation_started_unix", "operation_created_unix", "id")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "queued_timeout", "operation_started_unix", "operation_created_unix", "id")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "running_timeout", "operation_deadline_unix", "id")
	assertIndexColumns(t, (&Codespace{}).TableIndices(), "failed_retention", "status", "updated_unix", "id")
}

func TestManagerTableIndices(t *testing.T) {
	assertIndexColumns(t, (&Manager{}).TableIndices(), "user", "user_id")
}

func TestValidateCodespace(t *testing.T) {
	for _, status := range []string{StatusRunning, StatusStopped, StatusFailed} {
		t.Run("status/"+status, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.Status = status
			require.NoError(t, ValidateCodespace(row))
		})
	}

	for status, operationType := range map[string]string{
		StatusCreating: OperationCreate,
		StatusStopped:  OperationResume,
		StatusRunning:  OperationStop,
		StatusDeleting: OperationDelete,
	} {
		t.Run("operation type/"+operationType, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.OperationTrigger = OperationTriggerUser
			row.OperationCreatedUnix = 1
			row.Status = status
			require.NoError(t, ValidateCodespace(row))
			assert.Equal(t, operationType, ActiveOperationType(row))
			assert.True(t, IsOperationQueued(row))
		})
	}

	for _, trigger := range []string{OperationTriggerUser, OperationTriggerIdle} {
		t.Run("operation trigger/"+trigger, func(t *testing.T) {
			row := validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			row.OperationTrigger = trigger
			row.OperationCreatedUnix = 1
			row.Status = StatusRunning
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
	row.Status = "invalid"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.OperationTrigger = OperationTriggerUser
	row.OperationCreatedUnix = 1
	row.Status = StatusCreating
	require.NoError(t, ValidateCodespace(row))

	row.OperationStartedUnix = 2
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.OperationTrigger = OperationTriggerUser
	row.OperationCreatedUnix = 1
	row.Status = StatusFailed
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.OperationTrigger = "timer"
	row.OperationCreatedUnix = 1
	row.Status = StatusCreating
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.AutoStopMode = "disabled"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.DevContainerPath = ".devcontainer/devcontainer.json"
	assert.Error(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.DevContainerPath = ".devcontainer/devcontainer.json"
	row.DevContainerContent = ""
	require.NoError(t, ValidateCodespace(row))

	row = validCodespace("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	row.DevContainerPath = ""
	row.DevContainerContent = `{"image":"debian:12"}`
	require.NoError(t, ValidateCodespace(row))
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
		UUID:                codespaceUUID,
		UserID:              1,
		RepoID:              2,
		RefType:             "branch",
		RefName:             "main",
		EnvironmentTag:      "default",
		CommitSHA:           "0123456789abcdef0123456789abcdef01234567",
		DevContainerContent: `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		OperationRVersion:   1,
		ManagerID:           1,
		Status:              StatusStopped,
		AutoStopMode:        AutoStopModeDefault,
		CreatedUnix:         1,
		UpdatedUnix:         1,
		LogSize:             0,
		LastActiveUnix:      0,
	}
}
