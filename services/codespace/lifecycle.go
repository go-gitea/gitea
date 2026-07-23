// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
)

// FinalizeOutcome is the stable result returned by FinalizeOperation.
type FinalizeOutcome string

const (
	// FinalizeOutcomeAccepted means this request completed the active operation.
	FinalizeOutcomeAccepted FinalizeOutcome = "accepted"
	// FinalizeOutcomeIdempotent means the requested final state already exists.
	FinalizeOutcomeIdempotent FinalizeOutcome = "idempotent"
	// FinalizeOutcomeStale means the request no longer matches current state.
	FinalizeOutcomeStale FinalizeOutcome = "stale"
	// FinalizeOutcomeResourceAbsent means the Codespace row no longer exists.
	FinalizeOutcomeResourceAbsent FinalizeOutcome = "resource_absent"
)

// ErrFinalizeMetadataRequired is returned until current-version ready metadata is available.
var ErrFinalizeMetadataRequired = errors.New("current ready runtime metadata is required")

// ErrFinalizeGiteaTokenRequired is returned when final done lacks a Codespace token.
var ErrFinalizeGiteaTokenRequired = errors.New("codespace gitea token is required")

// FinalizeOperationOptions contains a Manager final report after protobuf enum validation.
type FinalizeOperationOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
	OperationType     string
	FinalStatus       string
}

const (
	// FinalStatusDone means the Manager completed the requested operation.
	FinalStatusDone = "done"
	// FinalStatusFailed means the Manager finished the operation with a failed result.
	FinalStatusFailed = "failed"
)

// FinalizeOperation applies a Manager final report to the current active operation.
func FinalizeOperation(ctx context.Context, manager *codespace_model.Manager, opts FinalizeOperationOptions) (FinalizeOutcome, error) {
	if manager == nil || manager.ID <= 0 {
		return "", errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return "", err
	}
	if opts.OperationRVersion <= 0 {
		return "", errors.New("operation_rversion must be positive")
	}
	if !isFinalOperationType(opts.OperationType) {
		return "", fmt.Errorf("invalid final operation type %q", opts.OperationType)
	}
	if opts.FinalStatus != FinalStatusDone && opts.FinalStatus != FinalStatusFailed {
		return "", fmt.Errorf("invalid final status %q", opts.FinalStatus)
	}

	var outcome FinalizeOutcome
	var summary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			outcome = FinalizeOutcomeResourceAbsent
			return nil
		}

		if isCurrentRunningOperation(codespace, manager.ID, opts.OperationRVersion) {
			if codespace.OperationType != opts.OperationType {
				outcome = FinalizeOutcomeStale
				return nil
			}
			now := time.Now().Unix()
			if codespace.OperationDeadlineUnix > 0 && now >= codespace.OperationDeadlineUnix {
				resultStatus := timeoutStatus(codespace.OperationType)
				summary = operationTimeoutSummary(codespace, resultStatus)
				if err := applyFinalState(ctx, codespace, resultStatus, now); err != nil {
					return err
				}
				outcome = matchingFinalTarget(opts, resultStatus)
				return nil
			}
			if requiresReadyMetadata(opts) {
				if err := requireFinalizeReadyPrerequisites(ctx, opts.CodespaceUUID, opts.OperationRVersion); err != nil {
					return err
				}
			}
			resultStatus := finalTargetStatus(opts)
			if resultStatus != "" {
				summary = operationFinalSummary(codespace, opts.FinalStatus, resultStatus)
			}
			if err := applyFinalOperation(ctx, codespace, opts, now); err != nil {
				return err
			}
			outcome = FinalizeOutcomeAccepted
			return nil
		}

		if codespace.OperationRVersion == opts.OperationRVersion && !hasActiveOperation(codespace) {
			outcome = matchingFinalTarget(opts, codespace.Status)
			return nil
		}

		outcome = FinalizeOutcomeStale
		return nil
	})
	if err != nil {
		return "", err
	}
	appendInternalStateSummary(ctx, summary)
	return outcome, nil
}

func isFinalOperationType(operationType string) bool {
	switch operationType {
	case codespace_model.OperationCreate, codespace_model.OperationResume, codespace_model.OperationStop, codespace_model.OperationDelete:
		return true
	default:
		return false
	}
}

func isCurrentRunningOperation(codespace *codespace_model.Codespace, managerID, operationRVersion int64) bool {
	return codespace.ManagerID == managerID &&
		codespace.OperationRVersion == operationRVersion &&
		codespace.OperationStatus == codespace_model.OperationStatusRunning
}

func hasActiveOperation(codespace *codespace_model.Codespace) bool {
	return codespace.OperationType != "" || codespace.OperationStatus != "" || codespace.OperationTrigger != ""
}

func requiresReadyMetadata(opts FinalizeOperationOptions) bool {
	return opts.FinalStatus == FinalStatusDone &&
		(opts.OperationType == codespace_model.OperationCreate || opts.OperationType == codespace_model.OperationResume)
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
	case FinalStatusDone:
		switch opts.OperationType {
		case codespace_model.OperationCreate:
			return applyFinalState(ctx, codespace, codespace_model.StatusRunning, now)
		case codespace_model.OperationResume:
			codespace.LastActiveUnix = now
			return applyFinalState(ctx, codespace, codespace_model.StatusRunning, now)
		case codespace_model.OperationStop:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		case codespace_model.OperationDelete:
			return deleteCodespaceForFinal(ctx, codespace.UUID)
		}
	case FinalStatusFailed:
		switch opts.OperationType {
		case codespace_model.OperationCreate, codespace_model.OperationStop, codespace_model.OperationDelete:
			return applyFinalState(ctx, codespace, codespace_model.StatusFailed, now)
		case codespace_model.OperationResume:
			return applyFinalState(ctx, codespace, codespace_model.StatusStopped, now)
		}
	}
	return errors.New("unsupported final result")
}

func timeoutStatus(operationType string) string {
	switch operationType {
	case codespace_model.OperationResume, codespace_model.OperationStop:
		return codespace_model.StatusStopped
	default:
		return codespace_model.StatusFailed
	}
}

func matchingFinalTarget(opts FinalizeOperationOptions, currentStatus string) FinalizeOutcome {
	if finalTargetStatus(opts) == currentStatus {
		return FinalizeOutcomeIdempotent
	}
	return FinalizeOutcomeStale
}

func finalTargetStatus(opts FinalizeOperationOptions) string {
	if opts.FinalStatus == FinalStatusDone {
		switch opts.OperationType {
		case codespace_model.OperationCreate, codespace_model.OperationResume:
			return codespace_model.StatusRunning
		case codespace_model.OperationStop:
			return codespace_model.StatusStopped
		case codespace_model.OperationDelete:
			return ""
		}
	}
	switch opts.OperationType {
	case codespace_model.OperationResume:
		return codespace_model.StatusStopped
	default:
		return codespace_model.StatusFailed
	}
}

func applyFinalState(ctx context.Context, codespace *codespace_model.Codespace, status string, now int64) error {
	codespace.Status = status
	codespace.UpdatedUnix = now
	if status == codespace_model.StatusStopped {
		codespace.StoppedUnix = now
	}
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
		"stopped_unix",
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
