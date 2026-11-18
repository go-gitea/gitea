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
	"gitea.dev/modules/util"
)

// ErrFinalizeGiteaTokenRequired is returned when final done lacks a Codespace token.
var ErrFinalizeGiteaTokenRequired = errors.New("codespace gitea token is required")

// FinalizeOperationOptions contains a Manager final report.
type FinalizeOperationOptions struct {
	CodespaceID       int64
	OperationRVersion int64
	FinalStatus       codespacev1.FinalStatus
}

// FinalizeOperation applies a Manager final report to the current active operation.
func FinalizeOperation(ctx context.Context, manager *codespace_model.Manager, opts FinalizeOperationOptions) (*codespacev1.FinalizeOperationResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if opts.CodespaceID <= 0 {
		return nil, util.NewInvalidArgumentErrorf("codespace_id must be positive")
	}
	if opts.OperationRVersion <= 0 {
		return nil, util.NewInvalidArgumentErrorf("operation_rversion must be positive")
	}
	if opts.FinalStatus != codespacev1.FinalStatus_FINAL_STATUS_DONE && opts.FinalStatus != codespacev1.FinalStatus_FINAL_STATUS_FAILED {
		return nil, util.NewInvalidArgumentErrorf("invalid final status %d", opts.FinalStatus)
	}
	response := &codespacev1.FinalizeOperationResponse{}
	var stateSummary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Omit("secrets_encrypted").Get(codespace)
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
		if !isCurrentRunningOperation(codespace, manager.ID, opts.OperationRVersion) {
			return nil
		}
		operationType := codespace_model.ActiveOperationType(codespace)
		now := time.Now().Unix()
		if isConvergentOperation(operationType) {
			if opts.FinalStatus == codespacev1.FinalStatus_FINAL_STATUS_FAILED {
				return util.NewInvalidArgumentErrorf("stop and delete operations must converge or let their lease expire")
			}
			return applyFinalOperation(ctx, codespace, operationType, opts.FinalStatus, now)
		}
		if codespace.OperationDeadlineUnix > 0 && now >= codespace.OperationDeadlineUnix {
			resultStatus := timeoutStatus(operationType)
			stateSummary = operationTimeoutSummary(codespace, resultStatus)
			return applyFinalState(ctx, codespace, resultStatus, now)
		}
		if opts.FinalStatus == codespacev1.FinalStatus_FINAL_STATUS_DONE &&
			(operationType == codespace_model.OperationCreate || operationType == codespace_model.OperationResume) {
			hasToken, err := hasValidCurrentGiteaToken(ctx, codespace.ID)
			if err != nil {
				return err
			}
			if !hasToken {
				return ErrFinalizeGiteaTokenRequired
			}
		}
		return applyFinalOperation(ctx, codespace, operationType, opts.FinalStatus, now)
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

func hasActiveStartup(codespace *codespace_model.Codespace, now int64) bool {
	operation := codespace_model.ActiveOperationType(codespace)
	return (operation == codespace_model.OperationCreate || operation == codespace_model.OperationResume) && codespace_model.IsOperationRunning(codespace) && codespace.OperationDeadlineUnix > now
}

func applyFinalOperation(ctx context.Context, codespace *codespace_model.Codespace, operationType string, finalStatus codespacev1.FinalStatus, now int64) error {
	switch finalStatus {
	case codespacev1.FinalStatus_FINAL_STATUS_DONE:
		switch operationType {
		case codespace_model.OperationCreate:
			return applyFinalState(ctx, codespace, codespace_model.StatusRunning, now)
		case codespace_model.OperationResume:
			codespace.LastActiveUnix = now
			return applyFinalState(ctx, codespace, codespace_model.StatusRunning, now)
		case codespace_model.OperationStop:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		case codespace_model.OperationDelete:
			return deleteCodespaceRowForFinal(ctx, codespace)
		}
	case codespacev1.FinalStatus_FINAL_STATUS_FAILED:
		switch operationType {
		case codespace_model.OperationCreate:
			return applyFinalState(ctx, codespace, codespace_model.StatusFailed, now)
		case codespace_model.OperationResume:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		}
	}
	return errors.New("unsupported final result")
}

func isConvergentOperation(operationType string) bool {
	return operationType == codespace_model.OperationStop || operationType == codespace_model.OperationDelete
}

func applyRunningTimeout(ctx context.Context, codespace *codespace_model.Codespace, now int64) (*internalStateSummary, error) {
	operationType := codespace_model.ActiveOperationType(codespace)
	if isConvergentOperation(operationType) {
		summary := operationRetrySummary(codespace)
		return summary, retryConvergentOperation(ctx, codespace, now)
	}
	status := timeoutStatus(operationType)
	summary := operationTimeoutSummary(codespace, status)
	return summary, applyFinalState(ctx, codespace, status, now)
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

func applyFinalState(ctx context.Context, codespace *codespace_model.Codespace, status string, now int64) error {
	expected := snapshotCodespaceState(codespace)
	codespace.Status = status
	if status == codespace_model.StatusRunning {
		codespace.RuntimeOperationRVersion = codespace.OperationRVersion
	} else {
		codespace.RuntimeOperationRVersion = 0
	}
	codespace.UpdatedUnix = now
	clearActiveOperation(codespace)
	if err := cleanupCredentialsForStatus(ctx, codespace, status); err != nil {
		return err
	}
	err := updateCodespaceIfCurrent(ctx, expected, codespace,
		"manager_id",
		"status",
		"runtime_operation_r_version",
		"operation_trigger",
		"operation_created_unix",
		"operation_started_unix",
		"operation_deadline_unix",
		"updated_unix",
		"last_active_unix",
	)
	if err == nil && status != codespace_model.StatusRunning {
		deleteRuntimeMetadata(codespace.ID)
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
		"id = ? AND manager_id = ? AND status = ? AND operation_r_version = ? AND runtime_operation_r_version = ? AND operation_trigger = ? AND operation_created_unix = ? AND operation_started_unix = ? AND operation_deadline_unix = ?",
		expected.id, expected.managerID, expected.status, expected.operationRVersion, expected.runtimeOperationRVersion, expected.operationTrigger,
		expected.operationCreatedUnix, expected.operationStartedUnix, expected.operationDeadlineUnix,
	)
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
	if err := deleteCodespaceLog(ctx, codespace.ID); err != nil {
		return err
	}
	deleteRuntimeMetadata(codespace.ID)
	return nil
}

func deleteGiteaToken(ctx context.Context, codespaceID int64) error {
	_, err := db.GetEngine(ctx).ID(codespaceID).Delete(new(codespace_model.GiteaToken))
	return err
}
