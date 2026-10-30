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
	"gitea.dev/modules/setting"
)

var (
	// ErrLifecycleActionNotFound is returned when the Codespace no longer exists.
	ErrLifecycleActionNotFound = errors.New("codespace not found")
	// ErrLifecycleActionPermissionDenied is returned when the user is not the Codespace creator.
	ErrLifecycleActionPermissionDenied = errors.New("codespace permission denied")
	// ErrLifecycleActionStateUnavailable is returned when the lifecycle state cannot accept the action.
	ErrLifecycleActionStateUnavailable = errors.New("codespace lifecycle state unavailable")
	// ErrLifecycleActionVersionExhausted is returned when operation_rversion cannot advance.
	ErrLifecycleActionVersionExhausted = errors.New("codespace operation version exhausted")
)

// LifecycleActionOptions identifies one creator lifecycle request.
type LifecycleActionOptions struct {
	UserID      int64
	CodespaceID int64
}

// LifecycleActionResult contains the accepted operation state.
type LifecycleActionResult struct {
	Status            string
	OperationType     string
	OperationRVersion int64
	Deleted           bool
}

// StopCodespace queues a user stop operation for a running Codespace.
func StopCodespace(ctx context.Context, opts LifecycleActionOptions) (*LifecycleActionResult, error) {
	return applyCreatorLifecycleAction(ctx, opts, codespace_model.OperationStop)
}

// ResumeCodespace queues a user resume operation for a stopped Codespace.
func ResumeCodespace(ctx context.Context, opts LifecycleActionOptions) (*LifecycleActionResult, error) {
	if !setting.Codespace.Enabled {
		return nil, ErrLifecycleActionStateUnavailable
	}
	return applyCreatorLifecycleAction(ctx, opts, codespace_model.OperationResume)
}

// DeleteCodespace deletes an unbound Codespace or queues a bound delete operation.
func DeleteCodespace(ctx context.Context, opts LifecycleActionOptions) (*LifecycleActionResult, error) {
	return applyCreatorLifecycleAction(ctx, opts, codespace_model.OperationDelete)
}

func applyCreatorLifecycleAction(ctx context.Context, opts LifecycleActionOptions, operationType string) (*LifecycleActionResult, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	if opts.CodespaceID <= 0 {
		return nil, errors.New("codespace_id must be positive")
	}

	var result *LifecycleActionResult
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			return ErrLifecycleActionNotFound
		}
		if codespace.UserID != opts.UserID {
			return ErrLifecycleActionPermissionDenied
		}
		now := time.Now().Unix()
		switch operationType {
		case codespace_model.OperationStop:
			result, err = applyStopAction(ctx, codespace, now)
		case codespace_model.OperationResume:
			if codespace.Status != codespace_model.StatusStopped || hasActiveOperation(codespace) || codespace.ManagerID <= 0 {
				return ErrLifecycleActionStateUnavailable
			}
			if err := queueLifecycleOperation(ctx, codespace, codespace_model.StatusStopped, now, true); err != nil {
				return err
			}
			result = lifecycleActionResult(codespace)
		case codespace_model.OperationDelete:
			result, err = applyDeleteAction(ctx, codespace, now)
		default:
			err = fmt.Errorf("unsupported lifecycle operation %q", operationType)
		}
		return err
	})
	if err != nil {
		if errors.Is(err, errCodespaceStateChanged) {
			return nil, ErrLifecycleActionStateUnavailable
		}
		return nil, err
	}
	return result, nil
}

func applyStopAction(ctx context.Context, codespace *codespace_model.Codespace, now int64) (*LifecycleActionResult, error) {
	if codespace.Status != codespace_model.StatusRunning {
		return nil, ErrLifecycleActionStateUnavailable
	}
	if hasActiveOperation(codespace) && !isQueuedIdleStop(codespace) {
		return nil, ErrLifecycleActionStateUnavailable
	}
	if isQueuedIdleStop(codespace) {
		expected := snapshotCodespaceState(codespace)
		codespace.OperationTrigger = codespace_model.OperationTriggerUser
		if err := updateCodespaceIfCurrent(ctx, expected, codespace, "operation_trigger"); err != nil {
			return nil, err
		}
		return lifecycleActionResult(codespace), nil
	}
	if err := queueLifecycleOperation(ctx, codespace, codespace_model.StatusRunning, now, false); err != nil {
		return nil, err
	}
	return lifecycleActionResult(codespace), nil
}

func applyDeleteAction(ctx context.Context, codespace *codespace_model.Codespace, now int64) (*LifecycleActionResult, error) {
	if codespace.ManagerID <= 0 {
		if codespace.Status != codespace_model.StatusCreating && codespace.Status != codespace_model.StatusFailed {
			return nil, ErrLifecycleActionStateUnavailable
		}
		if err := deleteCodespaceRowForFinal(ctx, codespace); err != nil {
			return nil, err
		}
		return &LifecycleActionResult{Deleted: true}, nil
	}
	if codespace.Status == codespace_model.StatusDeleting && hasActiveOperation(codespace) {
		return lifecycleActionResult(codespace), nil
	}
	if err := cleanupCredentialsForStatus(ctx, codespace, codespace_model.StatusDeleting); err != nil {
		return nil, err
	}
	if err := queueLifecycleOperation(ctx, codespace, codespace_model.StatusDeleting, now, false); err != nil {
		return nil, err
	}
	deleteRuntimeMetadata(codespace.UUID)
	return lifecycleActionResult(codespace), nil
}

func queueLifecycleOperation(ctx context.Context, codespace *codespace_model.Codespace, status string, now int64, advanceInteraction bool) error {
	expected := snapshotCodespaceState(codespace)
	nextVersion, err := codespace_model.NextVersion(codespace.OperationRVersion)
	if err != nil {
		return ErrLifecycleActionVersionExhausted
	}
	cols := []string{
		"status",
		"operation_r_version",
		"operation_trigger",
		"operation_created_unix",
		"operation_started_unix",
		"operation_deadline_unix",
		"updated_unix",
	}
	if advanceInteraction {
		nextInteractionGeneration, err := codespace_model.NextVersion(codespace.InteractionGeneration)
		if err != nil {
			return ErrLifecycleActionVersionExhausted
		}
		codespace.InteractionGeneration = nextInteractionGeneration
		codespace.LastActiveUnix = now
		cols = append(cols, "interaction_generation", "last_active_unix")
	}
	codespace.Status = status
	codespace.OperationRVersion = nextVersion
	codespace.OperationTrigger = codespace_model.OperationTriggerUser
	codespace.OperationCreatedUnix = now
	codespace.OperationStartedUnix = 0
	codespace.OperationDeadlineUnix = 0
	codespace.UpdatedUnix = now
	return updateCodespaceIfCurrent(ctx, expected, codespace, cols...)
}

func lifecycleActionResult(codespace *codespace_model.Codespace) *LifecycleActionResult {
	return &LifecycleActionResult{
		Status:            codespace.Status,
		OperationType:     codespace_model.ActiveOperationType(codespace),
		OperationRVersion: codespace.OperationRVersion,
	}
}
