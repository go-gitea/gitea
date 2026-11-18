// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
)

var errCodespaceStateChanged = errors.New("codespace state changed")

const codespaceStateColumns = "codespace.id, codespace.user_id, codespace.repo_id, " +
	"codespace.ref_type, codespace.ref_name, codespace.environment_tag, codespace.commit_sha, " +
	"codespace.permission_authorization_id, codespace.manager_id, codespace.status, " +
	"codespace.operation_r_version, codespace.runtime_operation_r_version, codespace.operation_trigger, " +
	"codespace.operation_created_unix, codespace.operation_started_unix, codespace.operation_deadline_unix, " +
	"codespace.last_active_unix, codespace.auto_stop_mode, codespace.auto_stop_timeout_seconds, " +
	"codespace.interaction_generation, codespace.created_unix, codespace.updated_unix, codespace.log_size"

type codespaceStateSnapshot struct {
	id                       int64
	managerID                int64
	status                   string
	operationRVersion        int64
	runtimeOperationRVersion int64
	operationTrigger         string
	operationCreatedUnix     int64
	operationStartedUnix     int64
	operationDeadlineUnix    int64
	interactionGeneration    int64
}

func snapshotCodespaceState(codespace *codespace_model.Codespace) codespaceStateSnapshot {
	return codespaceStateSnapshot{
		id:                       codespace.ID,
		managerID:                codespace.ManagerID,
		status:                   codespace.Status,
		operationRVersion:        codespace.OperationRVersion,
		runtimeOperationRVersion: codespace.RuntimeOperationRVersion,
		operationTrigger:         codespace.OperationTrigger,
		operationCreatedUnix:     codespace.OperationCreatedUnix,
		operationStartedUnix:     codespace.OperationStartedUnix,
		operationDeadlineUnix:    codespace.OperationDeadlineUnix,
		interactionGeneration:    codespace.InteractionGeneration,
	}
}

func updateCodespaceIfCurrent(ctx context.Context, expected codespaceStateSnapshot, codespace *codespace_model.Codespace, columns ...string) error {
	return updateCodespaceState(ctx, expected, codespace, false, columns...)
}

func updateCodespaceInteractionIfCurrent(ctx context.Context, expected codespaceStateSnapshot, codespace *codespace_model.Codespace, columns ...string) error {
	return updateCodespaceState(ctx, expected, codespace, true, columns...)
}

func updateCodespaceState(ctx context.Context, expected codespaceStateSnapshot, codespace *codespace_model.Codespace, matchInteraction bool, columns ...string) error {
	query := db.GetEngine(ctx).Where(
		"id = ? AND manager_id = ? AND status = ? AND operation_r_version = ? AND runtime_operation_r_version = ? AND operation_trigger = ? AND operation_created_unix = ? AND operation_started_unix = ? AND operation_deadline_unix = ?",
		expected.id, expected.managerID, expected.status, expected.operationRVersion, expected.runtimeOperationRVersion, expected.operationTrigger,
		expected.operationCreatedUnix, expected.operationStartedUnix, expected.operationDeadlineUnix,
	)
	if matchInteraction {
		query = query.And("interaction_generation = ?", expected.interactionGeneration)
	}
	affected, err := query.Cols(columns...).Update(codespace)
	if err != nil {
		return err
	}
	if affected != 1 {
		return errCodespaceStateChanged
	}
	return nil
}
