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
	"gitea.dev/modules/globallock"
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
	UserID        int64
	CodespaceUUID string
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
	if err := validateLifecycleActionOptions(opts); err != nil {
		return nil, err
	}

	var result *LifecycleActionResult
	err := globallock.LockAndDo(ctx, codespaceLifecycleActionLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace, err := loadLifecycleActionCodespace(ctx, opts)
			if err != nil {
				return err
			}
			now := time.Now().Unix()
			switch operationType {
			case codespace_model.OperationStop:
				result, err = applyStopAction(ctx, codespace, now)
			case codespace_model.OperationResume:
				result, err = applyResumeAction(ctx, codespace, now)
			case codespace_model.OperationDelete:
				result, err = applyDeleteAction(ctx, codespace, now)
			default:
				err = fmt.Errorf("unsupported lifecycle operation %q", operationType)
			}
			return err
		})
	})
	if err != nil {
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
		codespace.OperationTrigger = codespace_model.OperationTriggerUser
		if _, err := db.GetEngine(ctx).ID(codespace.UUID).Cols("operation_trigger").Update(codespace); err != nil {
			return nil, err
		}
		return lifecycleActionResult(codespace), nil
	}
	if err := queueLifecycleOperation(ctx, codespace, codespace_model.OperationStop, codespace_model.StatusRunning, now, false); err != nil {
		return nil, err
	}
	return lifecycleActionResult(codespace), nil
}

func applyResumeAction(ctx context.Context, codespace *codespace_model.Codespace, now int64) (*LifecycleActionResult, error) {
	if codespace.Status != codespace_model.StatusStopped || hasActiveOperation(codespace) || codespace.ManagerID <= 0 {
		return nil, ErrLifecycleActionStateUnavailable
	}
	if err := queueLifecycleOperation(ctx, codespace, codespace_model.OperationResume, codespace_model.StatusStopped, now, true); err != nil {
		return nil, err
	}
	return lifecycleActionResult(codespace), nil
}

func applyDeleteAction(ctx context.Context, codespace *codespace_model.Codespace, now int64) (*LifecycleActionResult, error) {
	if codespace.ManagerID <= 0 {
		if codespace.Status != codespace_model.StatusCreating && codespace.Status != codespace_model.StatusFailed {
			return nil, ErrLifecycleActionStateUnavailable
		}
		deleted, err := deleteUnboundCodespaceIfCurrent(ctx, codespace)
		if err != nil {
			return nil, err
		}
		if !deleted {
			return nil, ErrLifecycleActionStateUnavailable
		}
		return &LifecycleActionResult{Deleted: true}, nil
	}
	if codespace.Status == codespace_model.StatusDeleting && codespace.OperationType == codespace_model.OperationDelete {
		return lifecycleActionResult(codespace), nil
	}
	if err := cleanupCredentialsForStatus(ctx, codespace.UUID, codespace_model.StatusDeleting); err != nil {
		return nil, err
	}
	if err := queueLifecycleOperation(ctx, codespace, codespace_model.OperationDelete, codespace_model.StatusDeleting, now, false); err != nil {
		return nil, err
	}
	deleteRuntimeMetadata(codespace.UUID)
	return lifecycleActionResult(codespace), nil
}

func queueLifecycleOperation(ctx context.Context, codespace *codespace_model.Codespace, operationType, status string, now int64, advanceInteraction bool) error {
	nextVersion, err := codespace_model.NextVersion(codespace.OperationRVersion)
	if err != nil {
		return ErrLifecycleActionVersionExhausted
	}
	cols := []string{
		"status",
		"operation_r_version",
		"operation_type",
		"operation_status",
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
	codespace.OperationType = operationType
	codespace.OperationStatus = codespace_model.OperationStatusQueued
	codespace.OperationTrigger = codespace_model.OperationTriggerUser
	codespace.OperationCreatedUnix = now
	codespace.OperationStartedUnix = 0
	codespace.OperationDeadlineUnix = 0
	codespace.UpdatedUnix = now
	_, err = db.GetEngine(ctx).ID(codespace.UUID).Cols(cols...).Update(codespace)
	return err
}

func deleteUnboundCodespaceIfCurrent(ctx context.Context, codespace *codespace_model.Codespace) (bool, error) {
	affected, err := db.GetEngine(ctx).
		Where("uuid = ? AND user_id = ? AND manager_id = 0 AND status = ? AND operation_r_version = ? AND operation_type = ? AND operation_status = ? AND operation_trigger = ?",
			codespace.UUID,
			codespace.UserID,
			codespace.Status,
			codespace.OperationRVersion,
			codespace.OperationType,
			codespace.OperationStatus,
			codespace.OperationTrigger,
		).
		Delete(new(codespace_model.Codespace))
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := deleteGiteaToken(ctx, codespace.UUID); err != nil {
		return false, err
	}
	if err := deleteGitSSHKey(ctx, codespace.UUID); err != nil {
		return false, err
	}
	if err := deleteCodespaceLog(ctx, codespace.UUID); err != nil {
		return false, err
	}
	deleteRuntimeMetadata(codespace.UUID)
	return true, nil
}

func validateLifecycleActionOptions(opts LifecycleActionOptions) error {
	if opts.UserID <= 0 {
		return errors.New("user_id must be positive")
	}
	return codespace_model.ValidateUUID(opts.CodespaceUUID)
}

func loadLifecycleActionCodespace(ctx context.Context, opts LifecycleActionOptions) (*codespace_model.Codespace, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrLifecycleActionNotFound
	}
	if codespace.UserID != opts.UserID {
		return nil, ErrLifecycleActionPermissionDenied
	}
	return codespace, nil
}

func lifecycleActionResult(codespace *codespace_model.Codespace) *LifecycleActionResult {
	return &LifecycleActionResult{
		Status:            codespace.Status,
		OperationType:     codespace.OperationType,
		OperationRVersion: codespace.OperationRVersion,
	}
}

func codespaceLifecycleActionLockKey(codespaceUUID string) string {
	return codespaceInteractionLockKey(codespaceUUID)
}
