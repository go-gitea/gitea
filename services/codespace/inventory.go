// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/util"
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
		return nil, util.NewInvalidArgumentErrorf("%v", err)
	}
	stored, err := loadInventoryCodespaces(ctx, manager.ID, opts.Instances)
	if err != nil {
		return nil, err
	}
	if err := acceptInventoryGeneration(ctx, manager.ID, opts.InventoryGeneration); err != nil {
		return nil, err
	}

	reported := make(map[int64]struct{}, len(opts.Instances))
	response := &codespacev1.ReportInstancesResponse{Results: make([]*codespacev1.RuntimeInstanceResult, 0, len(opts.Instances))}
	for batch := range slices.Chunk(opts.Instances, db.DefaultMaxInSize) {
		if err := ensureInventoryGenerationCurrent(ctx, manager.ID, opts.InventoryGeneration); err != nil {
			return nil, err
		}
		for _, instance := range batch {
			reported[instance.GetCodespaceId()] = struct{}{}
			result, err := processReportedRuntimeInstance(ctx, manager.ID, opts.InventoryGeneration, instance, stored[instance.GetCodespaceId()])
			if err != nil {
				return nil, err
			}
			response.Results = append(response.Results, result)
		}
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
	seen := make(map[int64]struct{}, len(opts.Instances))
	for _, instance := range opts.Instances {
		if instance == nil {
			return errors.New("runtime instance is required")
		}
		if instance.GetCodespaceId() <= 0 {
			return errors.New("codespace_id must be positive")
		}
		if _, ok := seen[instance.GetCodespaceId()]; ok {
			return fmt.Errorf("duplicate codespace_id %d", instance.GetCodespaceId())
		}
		seen[instance.GetCodespaceId()] = struct{}{}
		switch instance.GetRuntimeState() {
		case codespacev1.RuntimeState_RUNTIME_STATE_CREATING,
			codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
			codespacev1.RuntimeState_RUNTIME_STATE_STOPPED:
		default:
			return fmt.Errorf("invalid runtime_state %d", instance.GetRuntimeState())
		}
		if instance.GetObservedOperationRversion() < 0 {
			return errors.New("observed_operation_rversion must not be negative")
		}
	}
	return nil
}

func loadInventoryCodespaces(ctx context.Context, managerID int64, instances []*codespacev1.RuntimeInstanceRef) (map[int64]*codespace_model.Codespace, error) {
	stored := make(map[int64]*codespace_model.Codespace, len(instances))
	for batch := range slices.Chunk(instances, db.DefaultMaxInSize) {
		ids := make([]int64, 0, len(batch))
		for _, instance := range batch {
			ids = append(ids, instance.GetCodespaceId())
		}
		var rows []*codespace_model.Codespace
		if err := db.GetEngine(ctx).In("id", ids).Select(codespaceStateColumns).Find(&rows); err != nil {
			return nil, err
		}
		for _, row := range rows {
			stored[row.ID] = row
		}
		for _, instance := range batch {
			codespace := stored[instance.GetCodespaceId()]
			if codespace != nil && codespace.ManagerID == managerID && instance.GetObservedOperationRversion() > codespace.OperationRVersion {
				return nil, ErrReportInstancesStateHistoryConflict
			}
		}
	}
	return stored, nil
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
		has, err := db.GetEngine(ctx).ID(managerID).Cols("inventory_generation").Get(manager)
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
	has, err := db.GetEngine(ctx).ID(managerID).Cols("inventory_generation").Get(manager)
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

func processReportedRuntimeInstance(ctx context.Context, managerID, inventoryGeneration int64, instance *codespacev1.RuntimeInstanceRef, codespace *codespace_model.Codespace) (*codespacev1.RuntimeInstanceResult, error) {
	result := &codespacev1.RuntimeInstanceResult{CodespaceId: instance.GetCodespaceId()}
	if codespace == nil {
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME
		return result, nil
	}
	if codespace.ManagerID != managerID {
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME
		return result, nil
	}
	if codespace.Status == codespace_model.StatusFailed {
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_CLEANUP_LOCAL_RUNTIME
		return result, nil
	}
	if err := codespace_model.ValidateCodespaceLifecycle(codespace); err != nil {
		return nil, fmt.Errorf("invalid persisted Codespace: %w", err)
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
		var summary *internalStateSummary
		err := db.WithTx(ctx, func(ctx context.Context) error {
			if err := ensureInventoryGenerationCurrent(ctx, managerID, inventoryGeneration); err != nil {
				return err
			}
			current := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(codespace.ID).Select(codespaceStateColumns).Get(current)
			if err != nil || !has {
				return err
			}
			if current.ManagerID != managerID || current.Status != codespace_model.StatusRunning || hasActiveOperation(current) {
				return nil
			}
			if err := codespace_model.ValidateCodespaceLifecycle(current); err != nil {
				return fmt.Errorf("invalid persisted Codespace: %w", err)
			}
			summary = &internalStateSummary{
				CodespaceID: current.ID,
				Message:     "Gitea recorded the reported runtime as stopped.",
			}
			return applyFinalState(ctx, current, codespace_model.StatusStopped, time.Now().Unix())
		})
		if err != nil {
			if errors.Is(err, errCodespaceStateChanged) {
				summary = nil
				return result, nil
			}
			return nil, err
		}
		appendInternalStateSummary(ctx, summary)
	case codespace.Status == codespace_model.StatusStopped && instance.GetRuntimeState() == codespacev1.RuntimeState_RUNTIME_STATE_RUNNING:
		result.Action = codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_STOP_LOCAL_RUNTIME
		result.CurrentOperationRversion = codespace.OperationRVersion
	}
	return result, nil
}

func processMissingRuntimeInstances(ctx context.Context, managerID, inventoryGeneration int64, reported map[int64]struct{}) error {
	var expected []int64
	if err := db.GetEngine(ctx).Table(new(codespace_model.Codespace)).
		Where("manager_id = ?", managerID).
		In("status", codespace_model.StatusCreating, codespace_model.StatusRunning, codespace_model.StatusStopped, codespace_model.StatusDeleting, codespace_model.StatusFailed).
		Cols("id").Find(&expected); err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, id := range expected {
		if _, ok := reported[id]; ok {
			continue
		}
		if err := processMissingRuntimeInstance(ctx, managerID, inventoryGeneration, id, now); err != nil {
			return err
		}
	}
	return nil
}

func processMissingRuntimeInstance(ctx context.Context, managerID, inventoryGeneration, codespaceID, now int64) error {
	var summary *internalStateSummary
	err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := ensureInventoryGenerationCurrent(ctx, managerID, inventoryGeneration); err != nil {
			return err
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(codespaceID).Select(codespaceStateColumns).Get(codespace)
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
			return applyFinalState(ctx, codespace, codespace_model.StatusFailed, now)
		case codespace_model.StatusRunning, codespace_model.StatusStopped:
			summary = runtimeMissingSummary(codespace)
			return applyFinalState(ctx, codespace, codespace_model.StatusFailed, now)
		case codespace_model.StatusDeleting:
			return deleteCodespaceRowForFinal(ctx, codespace)
		case codespace_model.StatusFailed:
			expected := snapshotCodespaceState(codespace)
			codespace.ManagerID = 0
			return updateCodespaceIfCurrent(ctx, expected, codespace, "manager_id")
		default:
			return nil
		}
	})
	if err != nil {
		if errors.Is(err, errCodespaceStateChanged) {
			return nil
		}
		return err
	}
	appendInternalStateSummary(ctx, summary)
	return nil
}
