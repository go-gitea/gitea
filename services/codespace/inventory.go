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
	"gitea.dev/modules/globallock"
)

const maxRuntimeInstancesPerInventory = 10000

var (
	// ErrReportInstancesManagerUnavailable is returned when the Manager row is unavailable.
	ErrReportInstancesManagerUnavailable = errors.New("manager is unavailable")
	// ErrReportInstancesStateHistoryConflict is returned when Manager reports an unexplained newer operation version.
	ErrReportInstancesStateHistoryConflict = errors.New("runtime inventory state history conflict")
)

// ReportInstancesOptions contains one complete Manager inventory request.
type ReportInstancesOptions struct {
	InventoryGeneration int64
	Instances           []*codespacev1.RuntimeInstanceRef
}

// ReportInstances accepts a complete Runtime inventory snapshot and returns reconciliation actions.
func ReportInstances(ctx context.Context, manager *codespace_model.Manager, opts ReportInstancesOptions) (*codespacev1.ReportInstancesResponse, error) {
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

	reported := make(map[string]*codespacev1.RuntimeInstanceRef, len(opts.Instances))
	response := &codespacev1.ReportInstancesResponse{Results: make([]*codespacev1.RuntimeInstanceResult, 0, len(opts.Instances))}
	for _, instance := range opts.Instances {
		reported[instance.GetCodespaceUuid()] = instance
		if err := ensureInventoryGenerationCurrent(ctx, manager.ID, opts.InventoryGeneration); err != nil {
			return nil, err
		}
		result, err := processReportedRuntimeInstance(ctx, manager.ID, instance)
		if err != nil {
			return nil, err
		}
		response.Results = append(response.Results, result)
	}
	if err := processMissingRuntimeInstances(ctx, manager.ID, opts.InventoryGeneration, reported); err != nil {
		return nil, err
	}
	if err := ensureInventoryGenerationCurrent(ctx, manager.ID, opts.InventoryGeneration); err != nil {
		return nil, err
	}
	return response, nil
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
		if instance == nil {
			return errors.New("runtime instance is required")
		}
		if err := codespace_model.ValidateUUID(instance.GetCodespaceUuid()); err != nil {
			return err
		}
		if _, ok := seen[instance.GetCodespaceUuid()]; ok {
			return fmt.Errorf("duplicate codespace_uuid %q", instance.GetCodespaceUuid())
		}
		seen[instance.GetCodespaceUuid()] = struct{}{}
		if !validRuntimeInstanceState(instance.GetRuntimeState()) {
			return fmt.Errorf("invalid runtime_state %d", instance.GetRuntimeState())
		}
		if instance.GetObservedOperationRversion() < 0 {
			return errors.New("observed_operation_rversion must not be negative")
		}
	}
	return nil
}

func validRuntimeInstanceState(state codespacev1.RuntimeState) bool {
	switch state {
	case codespacev1.RuntimeState_RUNTIME_STATE_CREATING,
		codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
		codespacev1.RuntimeState_RUNTIME_STATE_STOPPED,
		codespacev1.RuntimeState_RUNTIME_STATE_FAILED:
		return true
	default:
		return false
	}
}

func precheckInventoryObservedVersions(ctx context.Context, managerID int64, instances []*codespacev1.RuntimeInstanceRef) error {
	for _, instance := range instances {
		if instance.GetObservedOperationRversion() <= 0 {
			continue
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).Where("uuid = ?", instance.GetCodespaceUuid()).Get(codespace)
		if err != nil {
			return err
		}
		if has && codespace.ManagerID == managerID && instance.GetObservedOperationRversion() > codespace.OperationRVersion {
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

func processReportedRuntimeInstance(ctx context.Context, managerID int64, instance *codespacev1.RuntimeInstanceRef) (*codespacev1.RuntimeInstanceResult, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).Where("uuid = ?", instance.GetCodespaceUuid()).Get(codespace)
	if err != nil {
		return nil, err
	}
	result := &codespacev1.RuntimeInstanceResult{CodespaceUuid: instance.GetCodespaceUuid()}
	if !has {
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME
		return result, nil
	}
	if codespace.ManagerID != managerID {
		if codespace.ManagerID == 0 && codespace.Status == codespace_model.StatusCreating {
			return result, nil
		}
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME
		return result, nil
	}
	if codespace.Status == codespace_model.StatusFailed {
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME
		return result, nil
	}

	result.RuntimeSettings = runtimeSettingsMessage(effectiveRuntimeSettings(codespace))
	if hasActiveOperation(codespace) {
		switch {
		case instance.GetObservedOperationRversion() > 0 && instance.GetObservedOperationRversion() < codespace.OperationRVersion:
			result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REFETCH_OPERATION
			result.CurrentOperationRversion = codespace.OperationRVersion
		}
		return result, nil
	}
	if instance.GetObservedOperationRversion() > 0 {
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEAR_OPERATION_CONTEXT
		result.CurrentOperationRversion = codespace.OperationRVersion
		return result, nil
	}
	switch {
	case codespace.Status == codespace_model.StatusRunning && instance.GetRuntimeState() == codespacev1.RuntimeState_RUNTIME_STATE_STOPPED:
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REPORT_RUNTIME_TRANSITION
		result.CurrentOperationRversion = codespace.OperationRVersion
	case (codespace.Status == codespace_model.StatusRunning || codespace.Status == codespace_model.StatusStopped) && instance.GetRuntimeState() == codespacev1.RuntimeState_RUNTIME_STATE_FAILED:
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_REPORT_RUNTIME_TRANSITION
		result.CurrentOperationRversion = codespace.OperationRVersion
	case codespace.Status == codespace_model.StatusStopped && instance.GetRuntimeState() == codespacev1.RuntimeState_RUNTIME_STATE_RUNNING:
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_STOP_LOCAL_RUNTIME
		result.CurrentOperationRversion = codespace.OperationRVersion
	}
	return result, nil
}

func processMissingRuntimeInstances(ctx context.Context, managerID, inventoryGeneration int64, reported map[string]*codespacev1.RuntimeInstanceRef) error {
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
	err := globallock.LockAndDo(ctx, codespaceStateLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if err := ensureInventoryGenerationCurrent(ctx, managerID, inventoryGeneration); err != nil {
				return err
			}
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).Where("uuid = ?", codespaceUUID).Get(codespace)
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
	if err := cleanupCredentialsForStatus(ctx, codespace, codespace_model.StatusFailed); err != nil {
		return err
	}
	deleteRuntimeMetadata(codespace.UUID)
	_, err := db.GetEngine(ctx).ID(codespace.ID).Cols(
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
