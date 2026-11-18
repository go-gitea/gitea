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
)

const reconcileCodespacesBatchSize = 100

// ReconcileCodespacesOptions contains the retention policy for the Codespace maintenance task.
type ReconcileCodespacesOptions struct {
	FailedOlderThan time.Duration
}

// ReconcileCodespacesResult reports how many records were changed by the maintenance task.
type ReconcileCodespacesResult struct {
	RunningTimedOut   int
	FailedDeleted     int
	OpenTokensDeleted int
}

// ReconcileCodespaces applies database-only Codespace time boundaries.
func ReconcileCodespaces(ctx context.Context, opts ReconcileCodespacesOptions) (*ReconcileCodespacesResult, error) {
	if opts.FailedOlderThan <= 0 {
		return nil, errors.New("failed retention duration must be positive")
	}

	now := time.Now().Unix()
	result := &ReconcileCodespacesResult{}
	err := errors.Join(
		reconcileRunningOperationTimeouts(ctx, now, result),
		reconcileFailedCodespaces(ctx, now, opts.FailedOlderThan, result),
		deleteExpiredOpenTokens(ctx, now, result),
	)
	return result, err
}

func reconcileRunningOperationTimeouts(ctx context.Context, now int64, result *ReconcileCodespacesResult) error {
	var ids []int64
	if err := db.GetEngine(ctx).Table(new(codespace_model.Codespace)).
		Where("operation_created_unix > 0 AND operation_started_unix > 0 AND operation_deadline_unix > 0 AND operation_deadline_unix <= ?", now).
		Asc("operation_deadline_unix", "id").
		Limit(reconcileCodespacesBatchSize).
		Cols("id").Find(&ids); err != nil {
		return err
	}

	var errs []error
	for _, id := range ids {
		if err := reconcileRunningOperationTimeout(ctx, id, now, result); err != nil {
			log.Error("Failed to reconcile running Codespace operation %d: %v", id, err)
			errs = append(errs, fmt.Errorf("running operation %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func reconcileRunningOperationTimeout(ctx context.Context, codespaceID, now int64, result *ReconcileCodespacesResult) error {
	var summary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(codespaceID).Select(codespaceStateColumns).Get(codespace)
		if err != nil || !has {
			return err
		}
		if !codespace_model.IsOperationRunning(codespace) ||
			codespace.OperationDeadlineUnix <= 0 ||
			codespace.OperationDeadlineUnix > now {
			return nil
		}
		summary, err = applyRunningTimeout(ctx, codespace, now)
		if err != nil {
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
	var ids []int64
	if err := db.GetEngine(ctx).Table(new(codespace_model.Codespace)).
		Where("status = ? AND manager_id = ? AND operation_trigger = ? AND operation_created_unix = 0 AND operation_started_unix = 0 AND operation_deadline_unix = 0 AND updated_unix > 0 AND updated_unix <= ?",
			codespace_model.StatusFailed, 0, "", cutoff).
		Asc("updated_unix", "id").
		Limit(reconcileCodespacesBatchSize).
		Cols("id").Find(&ids); err != nil {
		return err
	}

	var errs []error
	for _, id := range ids {
		if err := reconcileFailedCodespace(ctx, id, cutoff, result); err != nil {
			log.Error("Failed to delete expired failed Codespace %d: %v", id, err)
			errs = append(errs, fmt.Errorf("failed codespace %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func reconcileFailedCodespace(ctx context.Context, codespaceID, cutoff int64, result *ReconcileCodespacesResult) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(codespaceID).Select(codespaceStateColumns).Get(codespace)
		if err != nil || !has {
			return err
		}
		if codespace.Status != codespace_model.StatusFailed || codespace.ManagerID != 0 || hasActiveOperation(codespace) ||
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

func deleteExpiredOpenTokens(ctx context.Context, now int64, result *ReconcileCodespacesResult) error {
	var hashes []string
	if err := db.GetEngine(ctx).Table(new(codespace_model.OpenToken)).
		Where("expires_unix <= ?", now).Asc("expires_unix", "code_hash").
		Limit(reconcileCodespacesBatchSize).Cols("code_hash").Find(&hashes); err != nil {
		return err
	}
	if len(hashes) == 0 {
		return nil
	}
	deleted, err := db.GetEngine(ctx).In("code_hash", hashes).Where("expires_unix <= ?", now).Delete(new(codespace_model.OpenToken))
	result.OpenTokensDeleted += int(deleted)
	return err
}
