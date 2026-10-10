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
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

const reconcileCodespacesBatchSize = 100

// ReconcileCodespacesOptions contains the retention policy for the Codespace maintenance task.
type ReconcileCodespacesOptions struct {
	FailedOlderThan time.Duration
}

// ReconcileCodespacesResult reports how many records were changed by the maintenance task.
type ReconcileCodespacesResult struct {
	QueuedTimedOut  int
	RunningTimedOut int
	FailedDeleted   int
}

// ReconcileCodespaces applies database-only Codespace time boundaries.
func ReconcileCodespaces(ctx context.Context, opts ReconcileCodespacesOptions) (*ReconcileCodespacesResult, error) {
	if opts.FailedOlderThan <= 0 {
		return nil, errors.New("failed retention duration must be positive")
	}

	now := time.Now().Unix()
	result := &ReconcileCodespacesResult{}
	err := errors.Join(
		reconcileQueuedOperationTimeouts(ctx, now, result),
		reconcileRunningOperationTimeouts(ctx, now, result),
		reconcileFailedCodespaces(ctx, now, opts.FailedOlderThan, result),
	)
	return result, err
}

func reconcileQueuedOperationTimeouts(ctx context.Context, now int64, result *ReconcileCodespacesResult) error {
	var rows []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("operation_created_unix > 0 AND operation_created_unix <= ? AND operation_started_unix = 0 AND operation_deadline_unix = 0",
			now-int64(setting.Codespace.QueueTimeout/time.Second)).
		In("status", codespace_model.StatusCreating, codespace_model.StatusStopped).
		Asc("operation_created_unix", "id").
		Limit(reconcileCodespacesBatchSize).
		Find(&rows); err != nil {
		return err
	}

	var errs []error
	for _, row := range rows {
		if err := reconcileQueuedOperationTimeout(ctx, row.UUID, now, result); err != nil {
			log.Error("Failed to reconcile queued Codespace operation %s: %v", row.UUID, err)
			errs = append(errs, fmt.Errorf("queued operation %s: %w", row.UUID, err))
		}
	}
	return errors.Join(errs...)
}

func reconcileQueuedOperationTimeout(ctx context.Context, codespaceUUID string, now int64, result *ReconcileCodespacesResult) error {
	var summary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).Where("uuid = ?", codespaceUUID).Get(codespace)
		if err != nil || !has {
			return err
		}
		if !codespace_model.IsOperationQueued(codespace) || !isQueuedExpired(codespace, time.Unix(now, 0)) {
			return nil
		}
		summary = operationTimeoutSummary(codespace, queuedTimeoutStatus(codespace_model.ActiveOperationType(codespace)))
		if err := applyQueuedTimeout(ctx, codespace, now); err != nil {
			if errors.Is(err, errCodespaceStateChanged) {
				summary = nil
				return nil
			}
			return err
		}
		result.QueuedTimedOut++
		return nil
	})
	if err != nil {
		return err
	}
	appendInternalStateSummary(ctx, summary)
	return nil
}

func reconcileRunningOperationTimeouts(ctx context.Context, now int64, result *ReconcileCodespacesResult) error {
	var rows []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("operation_created_unix > 0 AND operation_started_unix > 0 AND operation_deadline_unix > 0 AND operation_deadline_unix <= ?", now).
		Asc("operation_deadline_unix", "id").
		Limit(reconcileCodespacesBatchSize).
		Find(&rows); err != nil {
		return err
	}

	var errs []error
	for _, row := range rows {
		if err := reconcileRunningOperationTimeout(ctx, row.UUID, now, result); err != nil {
			log.Error("Failed to reconcile running Codespace operation %s: %v", row.UUID, err)
			errs = append(errs, fmt.Errorf("running operation %s: %w", row.UUID, err))
		}
	}
	return errors.Join(errs...)
}

func reconcileRunningOperationTimeout(ctx context.Context, codespaceUUID string, now int64, result *ReconcileCodespacesResult) error {
	var summary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).Where("uuid = ?", codespaceUUID).Get(codespace)
		if err != nil || !has {
			return err
		}
		if !codespace_model.IsOperationRunning(codespace) ||
			codespace.OperationDeadlineUnix <= 0 ||
			codespace.OperationDeadlineUnix > now {
			return nil
		}
		operationType := codespace_model.ActiveOperationType(codespace)
		summary = operationTimeoutSummary(codespace, timeoutStatus(operationType))
		if isConvergentOperation(operationType) {
			summary = operationRetrySummary(codespace)
		}
		if err := applyRunningTimeout(ctx, codespace, now); err != nil {
			if errors.Is(err, errCodespaceStateChanged) {
				summary = nil
				return nil
			}
			return err
		}
		result.RunningTimedOut++
		return nil
	})
	if err != nil {
		return err
	}
	appendInternalStateSummary(ctx, summary)
	return nil
}

func reconcileFailedCodespaces(ctx context.Context, now int64, olderThan time.Duration, result *ReconcileCodespacesResult) error {
	cutoff := now - int64(olderThan/time.Second)
	var rows []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("status = ? AND manager_id = ? AND uuid IS NULL AND operation_trigger = ? AND operation_created_unix = 0 AND operation_started_unix = 0 AND operation_deadline_unix = 0 AND updated_unix > 0 AND updated_unix <= ?",
			codespace_model.StatusFailed, 0, "", cutoff).
		Asc("updated_unix", "id").
		Limit(reconcileCodespacesBatchSize).
		Find(&rows); err != nil {
		return err
	}

	var errs []error
	for _, row := range rows {
		if err := reconcileFailedCodespace(ctx, row.ID, cutoff, result); err != nil {
			log.Error("Failed to delete expired failed Codespace %d: %v", row.ID, err)
			errs = append(errs, fmt.Errorf("failed codespace %d: %w", row.ID, err))
		}
	}
	return errors.Join(errs...)
}

func reconcileFailedCodespace(ctx context.Context, codespaceID, cutoff int64, result *ReconcileCodespacesResult) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(codespaceID).Get(codespace)
		if err != nil || !has {
			return err
		}
		if codespace.Status != codespace_model.StatusFailed || codespace.ManagerID != 0 || codespace.UUID != "" || hasActiveOperation(codespace) ||
			codespace.UpdatedUnix <= 0 || codespace.UpdatedUnix > cutoff {
			return nil
		}
		if err := deleteCodespaceRowForFinal(ctx, codespace); err != nil {
			if errors.Is(err, errCodespaceStateChanged) {
				return nil
			}
			return err
		}
		result.FailedDeleted++
		return nil
	})
}
