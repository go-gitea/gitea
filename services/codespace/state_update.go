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

type codespaceStateSnapshot struct {
	id                    int64
	uuid                  string
	managerID             int64
	status                string
	operationRVersion     int64
	operationTrigger      string
	operationCreatedUnix  int64
	operationStartedUnix  int64
	operationDeadlineUnix int64
	interactionGeneration int64
}

func snapshotCodespaceState(codespace *codespace_model.Codespace) codespaceStateSnapshot {
	return codespaceStateSnapshot{
		id:                    codespace.ID,
		uuid:                  codespace.UUID,
		managerID:             codespace.ManagerID,
		status:                codespace.Status,
		operationRVersion:     codespace.OperationRVersion,
		operationTrigger:      codespace.OperationTrigger,
		operationCreatedUnix:  codespace.OperationCreatedUnix,
		operationStartedUnix:  codespace.OperationStartedUnix,
		operationDeadlineUnix: codespace.OperationDeadlineUnix,
		interactionGeneration: codespace.InteractionGeneration,
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
		"id = ? AND manager_id = ? AND status = ? AND operation_r_version = ? AND operation_trigger = ? AND operation_created_unix = ? AND operation_started_unix = ? AND operation_deadline_unix = ?",
		expected.id, expected.managerID, expected.status, expected.operationRVersion, expected.operationTrigger,
		expected.operationCreatedUnix, expected.operationStartedUnix, expected.operationDeadlineUnix,
	)
	if matchInteraction {
		query = query.And("interaction_generation = ?", expected.interactionGeneration)
	}
	if expected.uuid == "" {
		query = query.And("uuid IS NULL")
	} else {
		query = query.And("uuid = ?", expected.uuid)
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
