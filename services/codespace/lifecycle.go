// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
)

// ErrFinalizeMetadataRequired is returned until current-version ready metadata is available.
var ErrFinalizeMetadataRequired = errors.New("current ready runtime metadata is required")

// ErrFinalizeGiteaTokenRequired is returned when final done lacks a Codespace token.
var ErrFinalizeGiteaTokenRequired = errors.New("codespace gitea token is required")

// FinalizeOperationOptions contains a Manager final report.
type FinalizeOperationOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
	OperationType     codespacev1.OperationType
	FinalStatus       codespacev1.FinalStatus
}

// FinalizeOperation applies a Manager final report to the current active operation.
func FinalizeOperation(ctx context.Context, manager *codespace_model.Manager, opts FinalizeOperationOptions) (*codespacev1.FinalizeOperationResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if opts.OperationRVersion <= 0 {
		return nil, errors.New("operation_rversion must be positive")
	}
	operationType := finalOperationType(opts.OperationType)
	if operationType == "" {
		return nil, fmt.Errorf("invalid final operation type %d", opts.OperationType)
	}
	if opts.FinalStatus != codespacev1.FinalStatus_FINAL_STATUS_DONE && opts.FinalStatus != codespacev1.FinalStatus_FINAL_STATUS_FAILED {
		return nil, fmt.Errorf("invalid final status %d", opts.FinalStatus)
	}
	response := &codespacev1.FinalizeOperationResponse{}
	var stateSummary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).Where("uuid = ?", opts.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			response.ResourceAbsent = true
			return nil
		}
		if err := codespace_model.ValidateCodespace(codespace); err != nil {
			return fmt.Errorf("invalid persisted Codespace: %w", err)
		}

		// A stale final ends Manager work but must not overwrite a newer operation, so acknowledge it without changing state.
		if !isCurrentRunningOperation(codespace, manager.ID, opts.OperationRVersion) || codespace_model.ActiveOperationType(codespace) != operationType {
			return nil
		}
		now := time.Now().Unix()
		if isConvergentOperation(operationType) {
			if opts.FinalStatus == codespacev1.FinalStatus_FINAL_STATUS_FAILED {
				return errors.New("stop and delete operations must converge or let their lease expire")
			}
			return applyFinalOperation(ctx, codespace, opts, now)
		}
		if codespace.OperationDeadlineUnix > 0 && now >= codespace.OperationDeadlineUnix {
			resultStatus := timeoutStatus(operationType)
			stateSummary = operationTimeoutSummary(codespace, resultStatus)
			return applyFinalState(ctx, codespace, resultStatus, now)
		}
		if opts.FinalStatus == codespacev1.FinalStatus_FINAL_STATUS_DONE &&
			(opts.OperationType == codespacev1.OperationType_OPERATION_TYPE_CREATE || opts.OperationType == codespacev1.OperationType_OPERATION_TYPE_RESUME) {
			if err := requireFinalizeReadyPrerequisites(ctx, codespace, opts.OperationRVersion); err != nil {
				return err
			}
		}
		return applyFinalOperation(ctx, codespace, opts, now)
	})
	if err != nil {
		if errors.Is(err, errCodespaceStateChanged) {
			return response, nil
		}
		return nil, err
	}
	appendInternalStateSummary(ctx, stateSummary)
	return response, nil
}

func isCurrentRunningOperation(codespace *codespace_model.Codespace, managerID, operationRVersion int64) bool {
	return codespace.ManagerID == managerID &&
		codespace.OperationRVersion == operationRVersion &&
		codespace_model.IsOperationRunning(codespace)
}

func hasActiveOperation(codespace *codespace_model.Codespace) bool {
	return codespace_model.ActiveOperationType(codespace) != ""
}

func requireFinalizeReadyPrerequisites(ctx context.Context, codespace *codespace_model.Codespace, operationRVersion int64) error {
	hasToken, err := hasValidCurrentGiteaToken(ctx, codespace.ID)
	if err != nil {
		return err
	}
	if !hasToken {
		return ErrFinalizeGiteaTokenRequired
	}
	hasMetadata, err := HasReadyRuntimeMetadata(ctx, codespace.UUID, operationRVersion)
	if err != nil {
		return err
	}
	if !hasMetadata {
		return ErrFinalizeMetadataRequired
	}
	return nil
}

func applyFinalOperation(ctx context.Context, codespace *codespace_model.Codespace, opts FinalizeOperationOptions, now int64) error {
	switch opts.FinalStatus {
	case codespacev1.FinalStatus_FINAL_STATUS_DONE:
		switch opts.OperationType {
		case codespacev1.OperationType_OPERATION_TYPE_CREATE:
			return applyFinalState(ctx, codespace, codespace_model.StatusRunning, now)
		case codespacev1.OperationType_OPERATION_TYPE_RESUME:
			codespace.LastActiveUnix = now
			return applyFinalState(ctx, codespace, codespace_model.StatusRunning, now)
		case codespacev1.OperationType_OPERATION_TYPE_STOP:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		case codespacev1.OperationType_OPERATION_TYPE_DELETE:
			return deleteCodespaceRowForFinal(ctx, codespace)
		}
	case codespacev1.FinalStatus_FINAL_STATUS_FAILED:
		switch opts.OperationType {
		case codespacev1.OperationType_OPERATION_TYPE_CREATE:
			return applyFinalState(ctx, codespace, codespace_model.StatusFailed, now)
		case codespacev1.OperationType_OPERATION_TYPE_RESUME:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		}
	}
	return errors.New("unsupported final result")
}

func isConvergentOperation(operationType string) bool {
	return operationType == codespace_model.OperationStop || operationType == codespace_model.OperationDelete
}

func retryConvergentOperation(ctx context.Context, codespace *codespace_model.Codespace, now int64) error {
	if !isConvergentOperation(codespace_model.ActiveOperationType(codespace)) {
		return errors.New("operation is not convergent")
	}
	expected := snapshotCodespaceState(codespace)
	nextVersion, err := codespace_model.NextVersion(codespace.OperationRVersion)
	if err != nil {
		return err
	}
	codespace.OperationRVersion = nextVersion
	codespace.OperationStartedUnix = 0
	codespace.OperationDeadlineUnix = 0
	codespace.UpdatedUnix = now
	err = updateCodespaceIfCurrent(ctx, expected, codespace,
		"operation_r_version",
		"operation_started_unix",
		"operation_deadline_unix",
		"updated_unix",
	)
	return err
}

func timeoutStatus(operationType string) string {
	switch operationType {
	case codespace_model.OperationResume:
		return codespace_model.StatusStopped
	default:
		return codespace_model.StatusFailed
	}
}

func finalOperationType(operationType codespacev1.OperationType) string {
	switch operationType {
	case codespacev1.OperationType_OPERATION_TYPE_CREATE:
		return codespace_model.OperationCreate
	case codespacev1.OperationType_OPERATION_TYPE_RESUME:
		return codespace_model.OperationResume
	case codespacev1.OperationType_OPERATION_TYPE_STOP:
		return codespace_model.OperationStop
	case codespacev1.OperationType_OPERATION_TYPE_DELETE:
		return codespace_model.OperationDelete
	default:
		return ""
	}
}

func applyFinalState(ctx context.Context, codespace *codespace_model.Codespace, status string, now int64) error {
	expected := snapshotCodespaceState(codespace)
	codespace.Status = status
	codespace.UpdatedUnix = now
	clearActiveOperation(codespace)
	if err := cleanupCredentialsForStatus(ctx, codespace, status); err != nil {
		return err
	}
	err := updateCodespaceIfCurrent(ctx, expected, codespace,
		"status",
		"operation_trigger",
		"operation_created_unix",
		"operation_started_unix",
		"operation_deadline_unix",
		"updated_unix",
		"last_active_unix",
	)
	if err == nil && status != codespace_model.StatusRunning {
		deleteRuntimeMetadata(codespace.UUID)
	}
	return err
}

func clearActiveOperation(codespace *codespace_model.Codespace) {
	codespace.OperationTrigger = ""
	codespace.OperationCreatedUnix = 0
	codespace.OperationStartedUnix = 0
	codespace.OperationDeadlineUnix = 0
}

func cleanupCredentialsForStatus(ctx context.Context, codespace *codespace_model.Codespace, status string) error {
	switch status {
	case codespace_model.StatusRunning:
		return nil
	case codespace_model.StatusStopped:
		return deleteGiteaToken(ctx, codespace.ID)
	case codespace_model.StatusFailed, codespace_model.StatusDeleting:
		if err := deleteGiteaToken(ctx, codespace.ID); err != nil {
			return err
		}
		return deleteGitSSHKey(ctx, codespace.ID)
	default:
		return nil
	}
}

func deleteCodespaceRowForFinal(ctx context.Context, codespace *codespace_model.Codespace) error {
	expected := snapshotCodespaceState(codespace)
	query := db.GetEngine(ctx).Where(
		"id = ? AND manager_id = ? AND status = ? AND operation_r_version = ? AND operation_trigger = ? AND operation_created_unix = ? AND operation_started_unix = ? AND operation_deadline_unix = ?",
		expected.id, expected.managerID, expected.status, expected.operationRVersion, expected.operationTrigger,
		expected.operationCreatedUnix, expected.operationStartedUnix, expected.operationDeadlineUnix,
	)
	if expected.uuid == "" {
		query = query.And("uuid IS NULL")
	} else {
		query = query.And("uuid = ?", expected.uuid)
	}
	affected, err := query.Delete(new(codespace_model.Codespace))
	if err != nil {
		return err
	}
	if affected != 1 {
		return errCodespaceStateChanged
	}
	if err := deleteGiteaToken(ctx, codespace.ID); err != nil {
		return err
	}
	if err := deleteGitSSHKey(ctx, codespace.ID); err != nil {
		return err
	}
	if _, err := db.GetEngine(ctx).Where("codespace_id = ?", codespace.ID).Delete(new(codespace_model.OpenToken)); err != nil {
		return err
	}
	if codespace.UUID != "" {
		if err := deleteCodespaceLog(ctx, codespace.UUID); err != nil {
			return err
		}
		deleteRuntimeMetadata(codespace.UUID)
	}
	return nil
}

func deleteGiteaToken(ctx context.Context, codespaceID int64) error {
	_, err := db.GetEngine(ctx).ID(codespaceID).Delete(new(codespace_model.GiteaToken))
	return err
}
