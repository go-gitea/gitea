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
)

const maxRuntimeInstancesPerInventory = 10000

var (
	// ErrReportInstancesManagerUnavailable is returned when the Manager row is unavailable.
	ErrReportInstancesManagerUnavailable = errors.New("manager is unavailable")
	// ErrReportInstancesStateHistoryConflict is returned when Manager reports an unexplained newer operation version.
	ErrReportInstancesStateHistoryConflict = errors.New("runtime inventory state history conflict")
)

const (
	// RuntimeInstanceStateCreating means the local Runtime identity exists but is still starting.
	RuntimeInstanceStateCreating = "creating"
	// RuntimeInstanceStateRunning means the local Runtime is running.
	RuntimeInstanceStateRunning = "running"
	// RuntimeInstanceStateStopped means the local Runtime is stopped but recoverable.
	RuntimeInstanceStateStopped = "stopped"
	// RuntimeInstanceStateFailed means the local Runtime is not recoverable.
	RuntimeInstanceStateFailed = "failed"
)

const (
	// InventoryActionCleanupLocalRuntime instructs Manager to delete the local Runtime.
	InventoryActionCleanupLocalRuntime = "cleanup_local_runtime"
	// InventoryActionReportRuntimeTransition asks Manager to submit a stopped or failed fact.
	InventoryActionReportRuntimeTransition = "report_runtime_transition"
	// InventoryActionRefetchOperation asks Manager to fetch the current operation payload.
	InventoryActionRefetchOperation = "refetch_operation"
	// InventoryActionStopLocalRuntime asks Manager to stop a running local Runtime.
	InventoryActionStopLocalRuntime = "stop_local_runtime"
	// InventoryActionClearOperationContext asks Manager to drop stale operation context.
	InventoryActionClearOperationContext = "clear_operation_context"
)

// RuntimeInstanceRef contains one local Runtime inventory item.
type RuntimeInstanceRef struct {
	CodespaceUUID             string
	RuntimeState              string
	ObservedOperationRVersion int64
}

// RuntimeInstanceResult contains Gitea's current decision for one reported Runtime.
type RuntimeInstanceResult struct {
	CodespaceUUID            string
	RuntimeSettings          *RuntimeSettings
	Action                   string
	CurrentOperationRVersion int64
}

// ReportInstancesOptions contains one complete Manager inventory request.
type ReportInstancesOptions struct {
	InventoryGeneration int64
	Instances           []RuntimeInstanceRef
}

// ReportInstancesResult contains one response result for each reported Runtime.
type ReportInstancesResult struct {
	Results []RuntimeInstanceResult
}

// ReportInstances accepts a complete Runtime inventory snapshot and returns reconciliation actions.
func ReportInstances(ctx context.Context, manager *codespace_model.Manager, opts ReportInstancesOptions) (*ReportInstancesResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := validateReportInstancesOptions(opts); err != nil {
		return nil, err
	}
	if err := precheckInventoryObservedVersions(ctx, manager.ID, opts.Instances); err != nil {
		return nil, err
	}
	if err := acceptInventoryGeneration(ctx, manager.ID, opts.InventoryGeneration); err != nil {
		return nil, err
	}

	reported := make(map[string]RuntimeInstanceRef, len(opts.Instances))
	results := make([]RuntimeInstanceResult, 0, len(opts.Instances))
	for _, instance := range opts.Instances {
		reported[instance.CodespaceUUID] = instance
		if err := ensureInventoryGenerationCurrent(ctx, manager.ID, opts.InventoryGeneration); err != nil {
			return nil, err
		}
		result, err := processReportedRuntimeInstance(ctx, manager.ID, instance)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := processMissingRuntimeInstances(ctx, manager.ID, opts.InventoryGeneration, reported); err != nil {
		return nil, err
	}
	if err := ensureInventoryGenerationCurrent(ctx, manager.ID, opts.InventoryGeneration); err != nil {
		return nil, err
	}
	return &ReportInstancesResult{Results: results}, nil
}

func validateReportInstancesOptions(opts ReportInstancesOptions) error {
	if opts.InventoryGeneration <= 0 {
		return errors.New("inventory_generation must be positive")
	}
	if len(opts.Instances) > maxRuntimeInstancesPerInventory {
		return fmt.Errorf("instances exceeds maximum %d", maxRuntimeInstancesPerInventory)
	}
	seen := make(map[string]struct{}, len(opts.Instances))
	for _, instance := range opts.Instances {
		if err := codespace_model.ValidateUUID(instance.CodespaceUUID); err != nil {
			return err
		}
		if _, ok := seen[instance.CodespaceUUID]; ok {
			return fmt.Errorf("duplicate codespace_uuid %q", instance.CodespaceUUID)
		}
		seen[instance.CodespaceUUID] = struct{}{}
		if !validRuntimeInstanceState(instance.RuntimeState) {
			return fmt.Errorf("invalid runtime_state %q", instance.RuntimeState)
		}
		if instance.ObservedOperationRVersion < 0 {
			return errors.New("observed_operation_rversion must not be negative")
		}
	}
	return nil
}

func validRuntimeInstanceState(state string) bool {
	switch state {
	case RuntimeInstanceStateCreating, RuntimeInstanceStateRunning, RuntimeInstanceStateStopped, RuntimeInstanceStateFailed:
		return true
	default:
		return false
	}
}

func precheckInventoryObservedVersions(ctx context.Context, managerID int64, instances []RuntimeInstanceRef) error {
	for _, instance := range instances {
		if instance.ObservedOperationRVersion <= 0 {
			continue
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(instance.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		if has && codespace.ManagerID == managerID && instance.ObservedOperationRVersion > codespace.OperationRVersion {
			return ErrReportInstancesStateHistoryConflict
		}
	}
	return nil
}

func acceptInventoryGeneration(ctx context.Context, managerID, inventoryGeneration int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		affected, err := db.GetEngine(ctx).
			Where("id = ? AND inventory_generation < ?", managerID, inventoryGeneration).
			Cols("inventory_generation").
			Update(&codespace_model.Manager{InventoryGeneration: inventoryGeneration})
		if err != nil {
			return err
		}
		if affected == 1 {
			return nil
		}

		manager := new(codespace_model.Manager)
		has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
		if err != nil {
			return err
		}
		if !has {
			return ErrReportInstancesManagerUnavailable
		}
		if manager.InventoryGeneration == inventoryGeneration {
			return nil
		}
		return &StaleGenerationError{CurrentGeneration: manager.InventoryGeneration}
	})
}

func ensureInventoryGenerationCurrent(ctx context.Context, managerID, inventoryGeneration int64) error {
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil {
		return err
	}
	if !has {
		return ErrReportInstancesManagerUnavailable
	}
	if manager.InventoryGeneration != inventoryGeneration {
		return &StaleGenerationError{CurrentGeneration: manager.InventoryGeneration}
	}
	return nil
}

func processReportedRuntimeInstance(ctx context.Context, managerID int64, instance RuntimeInstanceRef) (RuntimeInstanceResult, error) {
	var result RuntimeInstanceResult
	err := globallock.LockAndDo(ctx, inventoryCodespaceLockKey(instance.CodespaceUUID), func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(instance.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		result = runtimeInstanceResult(managerID, codespace, has, instance)
		return nil
	})
	return result, err
}

func runtimeInstanceResult(managerID int64, codespace *codespace_model.Codespace, has bool, instance RuntimeInstanceRef) RuntimeInstanceResult {
	result := RuntimeInstanceResult{CodespaceUUID: instance.CodespaceUUID}
	if !has {
		result.Action = InventoryActionCleanupLocalRuntime
		return result
	}
	if codespace.ManagerID != managerID {
		if codespace.ManagerID == 0 && codespace.Status == codespace_model.StatusCreating {
			return result
		}
		result.Action = InventoryActionCleanupLocalRuntime
		return result
	}
	if codespace.Status == codespace_model.StatusFailed {
		result.Action = InventoryActionCleanupLocalRuntime
		return result
	}

	settings := effectiveRuntimeSettings(codespace)
	result.RuntimeSettings = &settings
	if hasActiveOperation(codespace) {
		switch {
		case instance.ObservedOperationRVersion > 0 && instance.ObservedOperationRVersion < codespace.OperationRVersion:
			result.Action = InventoryActionRefetchOperation
			result.CurrentOperationRVersion = codespace.OperationRVersion
		}
		return result
	}
	if instance.ObservedOperationRVersion > 0 {
		result.Action = InventoryActionClearOperationContext
		result.CurrentOperationRVersion = codespace.OperationRVersion
		return result
	}
	switch {
	case codespace.Status == codespace_model.StatusRunning && instance.RuntimeState == RuntimeInstanceStateStopped:
		result.Action = InventoryActionReportRuntimeTransition
		result.CurrentOperationRVersion = codespace.OperationRVersion
	case (codespace.Status == codespace_model.StatusRunning || codespace.Status == codespace_model.StatusStopped) && instance.RuntimeState == RuntimeInstanceStateFailed:
		result.Action = InventoryActionReportRuntimeTransition
		result.CurrentOperationRVersion = codespace.OperationRVersion
	case codespace.Status == codespace_model.StatusStopped && instance.RuntimeState == RuntimeInstanceStateRunning:
		result.Action = InventoryActionStopLocalRuntime
		result.CurrentOperationRVersion = codespace.OperationRVersion
	}
	return result
}

func processMissingRuntimeInstances(ctx context.Context, managerID, inventoryGeneration int64, reported map[string]RuntimeInstanceRef) error {
	var expected []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("manager_id = ?", managerID).
		In("status", codespace_model.StatusCreating, codespace_model.StatusRunning, codespace_model.StatusStopped, codespace_model.StatusDeleting).
		Find(&expected); err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, codespace := range expected {
		if _, ok := reported[codespace.UUID]; ok {
			continue
		}
		if err := ensureInventoryGenerationCurrent(ctx, managerID, inventoryGeneration); err != nil {
			return err
		}
		if err := processMissingRuntimeInstance(ctx, managerID, inventoryGeneration, codespace.UUID, now); err != nil {
			return err
		}
	}
	return nil
}

func processMissingRuntimeInstance(ctx context.Context, managerID, inventoryGeneration int64, codespaceUUID string, now int64) error {
	var summary *internalStateSummary
	err := globallock.LockAndDo(ctx, inventoryCodespaceLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if err := ensureInventoryGenerationCurrent(ctx, managerID, inventoryGeneration); err != nil {
				return err
			}
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
			if err != nil || !has {
				return err
			}
			if codespace.ManagerID != managerID {
				return nil
			}
			switch codespace.Status {
			case codespace_model.StatusCreating:
				if currentOperationMatches(codespace, codespace_model.OperationCreate, codespace.OperationRVersion) &&
					codespace.OperationDeadlineUnix > now {
					return nil
				}
				summary = runtimeMissingSummary(codespace)
				return applyInventoryMissingFailed(ctx, codespace, now)
			case codespace_model.StatusRunning, codespace_model.StatusStopped:
				summary = runtimeMissingSummary(codespace)
				return applyInventoryMissingFailed(ctx, codespace, now)
			case codespace_model.StatusDeleting:
				return deleteCodespaceForFinal(ctx, codespace.UUID)
			default:
				return nil
			}
		})
	})
	if err != nil {
		return err
	}
	appendInternalStateSummary(ctx, summary)
	return nil
}

func applyInventoryMissingFailed(ctx context.Context, codespace *codespace_model.Codespace, now int64) error {
	codespace.Status = codespace_model.StatusFailed
	codespace.UpdatedUnix = now
	clearActiveOperation(codespace)
	if err := cleanupCredentialsForStatus(ctx, codespace.UUID, codespace_model.StatusFailed); err != nil {
		return err
	}
	deleteRuntimeMetadata(codespace.UUID)
	_, err := db.GetEngine(ctx).ID(codespace.UUID).Cols(
		"status",
		"operation_type",
		"operation_status",
		"operation_trigger",
		"operation_created_unix",
		"operation_started_unix",
		"operation_deadline_unix",
		"updated_unix",
	).Update(codespace)
	return err
}

func inventoryCodespaceLockKey(codespaceUUID string) string {
	return "codespace_inventory_" + codespaceUUID
}
