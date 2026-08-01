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
		has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			response.ResourceAbsent = true
			return nil
		}

		// A stale final ends Manager work but must not overwrite a newer operation, so acknowledge it without changing state.
		if !isCurrentRunningOperation(codespace, manager.ID, opts.OperationRVersion) || codespace.OperationType != operationType {
			return nil
		}
		now := time.Now().Unix()
		if codespace.OperationDeadlineUnix > 0 && now >= codespace.OperationDeadlineUnix {
			resultStatus := timeoutStatus(codespace.OperationType)
			stateSummary = operationTimeoutSummary(codespace, resultStatus)
			return applyFinalState(ctx, codespace, resultStatus, now)
		}
		if opts.FinalStatus == codespacev1.FinalStatus_FINAL_STATUS_DONE &&
			(opts.OperationType == codespacev1.OperationType_OPERATION_TYPE_CREATE || opts.OperationType == codespacev1.OperationType_OPERATION_TYPE_RESUME) {
			if err := requireFinalizeReadyPrerequisites(ctx, opts.CodespaceUUID, opts.OperationRVersion); err != nil {
				return err
			}
		}
		return applyFinalOperation(ctx, codespace, opts, now)
	})
	if err != nil {
		return nil, err
	}
	appendInternalStateSummary(ctx, stateSummary)
	return response, nil
}

func isCurrentRunningOperation(codespace *codespace_model.Codespace, managerID, operationRVersion int64) bool {
	return codespace.ManagerID == managerID &&
		codespace.OperationRVersion == operationRVersion &&
		codespace.OperationStatus == codespace_model.OperationStatusRunning
}

func hasActiveOperation(codespace *codespace_model.Codespace) bool {
	return codespace.OperationType != "" || codespace.OperationStatus != "" || codespace.OperationTrigger != ""
}

func requireFinalizeReadyPrerequisites(ctx context.Context, codespaceUUID string, operationRVersion int64) error {
	hasToken, err := hasValidCurrentGiteaToken(ctx, codespaceUUID)
	if err != nil {
		return err
	}
	if !hasToken {
		return ErrFinalizeGiteaTokenRequired
	}
	hasMetadata, err := HasReadyRuntimeMetadata(ctx, codespaceUUID, operationRVersion)
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
			return deleteCodespaceForFinal(ctx, codespace.UUID)
		}
	case codespacev1.FinalStatus_FINAL_STATUS_FAILED:
		switch opts.OperationType {
		case codespacev1.OperationType_OPERATION_TYPE_CREATE, codespacev1.OperationType_OPERATION_TYPE_STOP, codespacev1.OperationType_OPERATION_TYPE_DELETE:
			return applyFinalState(ctx, codespace, codespace_model.StatusFailed, now)
		case codespacev1.OperationType_OPERATION_TYPE_RESUME:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		}
	}
	return errors.New("unsupported final result")
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
	codespace.Status = status
	codespace.UpdatedUnix = now
	clearActiveOperation(codespace)
	if err := cleanupCredentialsForStatus(ctx, codespace.UUID, status); err != nil {
		return err
	}
	if status != codespace_model.StatusRunning {
		deleteRuntimeMetadata(codespace.UUID)
	}
	_, err := db.GetEngine(ctx).ID(codespace.UUID).Cols(
		"status",
		"operation_type",
		"operation_status",
		"operation_trigger",
		"operation_created_unix",
		"operation_started_unix",
		"operation_deadline_unix",
		"updated_unix",
		"last_active_unix",
	).Update(codespace)
	return err
}

func clearActiveOperation(codespace *codespace_model.Codespace) {
	codespace.OperationType = ""
	codespace.OperationStatus = ""
	codespace.OperationTrigger = ""
	codespace.OperationCreatedUnix = 0
	codespace.OperationStartedUnix = 0
	codespace.OperationDeadlineUnix = 0
}

func cleanupCredentialsForStatus(ctx context.Context, codespaceUUID, status string) error {
	switch status {
	case codespace_model.StatusRunning:
		return nil
	case codespace_model.StatusStopped:
		return deleteGiteaToken(ctx, codespaceUUID)
	case codespace_model.StatusFailed, codespace_model.StatusDeleting:
		if err := deleteGiteaToken(ctx, codespaceUUID); err != nil {
			return err
		}
		return deleteGitSSHKey(ctx, codespaceUUID)
	default:
		return nil
	}
}

func deleteCodespaceForFinal(ctx context.Context, codespaceUUID string) error {
	if err := deleteGiteaToken(ctx, codespaceUUID); err != nil {
		return err
	}
	if err := deleteGitSSHKey(ctx, codespaceUUID); err != nil {
		return err
	}
	if err := deleteCodespaceLog(ctx, codespaceUUID); err != nil {
		return err
	}
	deleteRuntimeMetadata(codespaceUUID)
	_, err := db.GetEngine(ctx).ID(codespaceUUID).Delete(new(codespace_model.Codespace))
	return err
}

func deleteGiteaToken(ctx context.Context, codespaceUUID string) error {
	_, err := db.GetEngine(ctx).Where("codespace_uuid = ?", codespaceUUID).Delete(new(codespace_model.GiteaToken))
	return err
}
