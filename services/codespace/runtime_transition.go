// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/globallock"
)

var (
	// ErrRuntimeTransitionNotFound is returned when the Codespace no longer exists.
	ErrRuntimeTransitionNotFound = errors.New("codespace not found")
	// ErrRuntimeTransitionManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrRuntimeTransitionManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrRuntimeTransitionCurrentOperationConflict is returned when an active operation owns the current lifecycle.
	ErrRuntimeTransitionCurrentOperationConflict = errors.New("current operation conflicts with runtime transition")
	// ErrRuntimeTransitionManagerOffline is returned when the authenticated Manager must declare before reporting.
	ErrRuntimeTransitionManagerOffline = errors.New("manager is offline")
	// ErrRuntimeTransitionStaleOperation is returned when the report cannot apply to the current lifecycle state.
	ErrRuntimeTransitionStaleOperation = errors.New("runtime transition is stale")
	// ErrRuntimeTransitionGenerationConflict is returned when one generation maps to a different target state.
	ErrRuntimeTransitionGenerationConflict = errors.New("runtime transition generation conflict")
)

const (
	// RuntimeTransitionStopped reports that the local Runtime is stopped and still recoverable.
	RuntimeTransitionStopped = "stopped"
	// RuntimeTransitionFailed reports that the local Runtime is not recoverable.
	RuntimeTransitionFailed = "failed"
)

// ReportRuntimeTransitionOptions contains one Manager runtime state fact.
type ReportRuntimeTransitionOptions struct {
	CodespaceUUID             string
	RuntimeGeneration         int64
	ObservedOperationRVersion int64
	RuntimeState              string
}

// ReportRuntimeTransition stores one stopped or failed Runtime state fact.
func ReportRuntimeTransition(ctx context.Context, manager *codespace_model.Manager, opts ReportRuntimeTransitionOptions) error {
	if manager == nil || manager.ID <= 0 {
		return errors.New("manager is required")
	}
	if err := validateRuntimeTransitionOptions(opts); err != nil {
		return err
	}

	var summary *internalStateSummary
	err := globallock.LockAndDo(ctx, runtimeTransitionLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				return ErrRuntimeTransitionNotFound
			}
			if codespace.ManagerID != manager.ID {
				return ErrRuntimeTransitionManagerMismatch
			}
			if transitionActiveOperationConflicts(codespace) {
				return ErrRuntimeTransitionCurrentOperationConflict
			}

			currentManager, err := loadFetchManager(ctx, manager.ID)
			if err != nil {
				return err
			}
			if !managerAllowsOnlineOrRecovering(currentManager) {
				return ErrRuntimeTransitionManagerOffline
			}
			if opts.ObservedOperationRVersion != codespace.OperationRVersion {
				return ErrRuntimeTransitionStaleOperation
			}
			if opts.RuntimeGeneration < codespace.RuntimeGeneration {
				return &StaleGenerationError{CurrentGeneration: codespace.RuntimeGeneration}
			}
			targetStatus := runtimeTransitionTargetStatus(opts.RuntimeState)
			if opts.RuntimeGeneration == codespace.RuntimeGeneration {
				if codespace.Status == targetStatus {
					return nil
				}
				return ErrRuntimeTransitionGenerationConflict
			}
			if !runtimeTransitionCompatible(codespace.Status, targetStatus) {
				return ErrRuntimeTransitionStaleOperation
			}
			if err := applyRuntimeTransition(ctx, codespace, targetStatus, opts.RuntimeGeneration, time.Now().Unix()); err != nil {
				return err
			}
			summary = runtimeTransitionSummary(codespace, opts.RuntimeGeneration, targetStatus)
			return nil
		})
	})
	if err != nil {
		return err
	}
	appendInternalStateSummary(ctx, summary)
	return nil
}

func validateRuntimeTransitionOptions(opts ReportRuntimeTransitionOptions) error {
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return err
	}
	if opts.RuntimeGeneration <= 0 {
		return errors.New("runtime_generation must be positive")
	}
	if opts.ObservedOperationRVersion <= 0 {
		return errors.New("observed_operation_rversion must be positive")
	}
	switch opts.RuntimeState {
	case RuntimeTransitionStopped, RuntimeTransitionFailed:
		return nil
	default:
		return errors.New("runtime_state must be stopped or failed")
	}
}

func transitionActiveOperationConflicts(codespace *codespace_model.Codespace) bool {
	return (codespace.Status == codespace_model.StatusRunning || codespace.Status == codespace_model.StatusStopped) &&
		hasActiveOperation(codespace)
}

func runtimeTransitionTargetStatus(runtimeState string) string {
	if runtimeState == RuntimeTransitionFailed {
		return codespace_model.StatusFailed
	}
	return codespace_model.StatusStopped
}

func runtimeTransitionCompatible(currentStatus, targetStatus string) bool {
	switch targetStatus {
	case codespace_model.StatusStopped:
		return currentStatus == codespace_model.StatusRunning
	case codespace_model.StatusFailed:
		return currentStatus == codespace_model.StatusRunning || currentStatus == codespace_model.StatusStopped
	default:
		return false
	}
}

func applyRuntimeTransition(ctx context.Context, codespace *codespace_model.Codespace, targetStatus string, runtimeGeneration, now int64) error {
	codespace.Status = targetStatus
	codespace.RuntimeGeneration = runtimeGeneration
	codespace.UpdatedUnix = now
	cols := []string{"status", "runtime_generation", "updated_unix"}
	if targetStatus == codespace_model.StatusStopped {
		codespace.StoppedUnix = now
		cols = append(cols, "stopped_unix")
	}
	if err := cleanupCredentialsForStatus(ctx, codespace.UUID, targetStatus); err != nil {
		return err
	}
	deleteRuntimeMetadata(codespace.UUID)
	_, err := db.GetEngine(ctx).ID(codespace.UUID).Cols(cols...).Update(codespace)
	return err
}

func runtimeTransitionLockKey(codespaceUUID string) string {
	return codespaceInteractionLockKey(codespaceUUID)
}
