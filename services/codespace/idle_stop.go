// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/util"
)

var (
	// ErrRequestIdleStopNotFound is returned when the Codespace no longer exists.
	ErrRequestIdleStopNotFound = errors.New("codespace not found")
	// ErrRequestIdleStopManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrRequestIdleStopManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrRequestIdleStopManagerUnavailable is returned when the authenticated Manager is not usable.
	ErrRequestIdleStopManagerUnavailable = errors.New("manager is not online")
	// ErrRequestIdleStopVersionExhausted is returned when operation_rversion cannot advance.
	ErrRequestIdleStopVersionExhausted = errors.New("codespace operation version exhausted")
)

// RequestIdleStopOptions contains one Manager idle-stop authorization request.
type RequestIdleStopOptions struct {
	CodespaceID                   int64
	ObservedIdleTimeoutSeconds    int64
	ObservedInteractionGeneration int64
}

// RequestIdleStop creates or confirms a queued idle stop after current setting validation.
func RequestIdleStop(ctx context.Context, manager *codespace_model.Manager, opts RequestIdleStopOptions) (*codespacev1.RequestIdleStopResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if opts.CodespaceID <= 0 {
		return nil, util.NewInvalidArgumentErrorf("codespace_id must be positive")
	}
	if opts.ObservedInteractionGeneration < 0 {
		return nil, util.NewInvalidArgumentErrorf("observed_interaction_generation must not be negative")
	}
	if opts.ObservedIdleTimeoutSeconds < 0 {
		return nil, util.NewInvalidArgumentErrorf("observed_idle_timeout_seconds must not be negative")
	}

	var response *codespacev1.RequestIdleStopResponse
	err := db.WithTx(ctx, func(ctx context.Context) error {
		currentManager, err := loadCodespaceManager(ctx, manager.ID)
		if err != nil {
			return err
		}
		if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
			return ErrRequestIdleStopManagerUnavailable
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Omit("secrets_encrypted").Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			return ErrRequestIdleStopNotFound
		}
		if codespace.ManagerID != manager.ID {
			return ErrRequestIdleStopManagerMismatch
		}

		if isQueuedIdleStop(codespace) {
			response = idleStopPendingResponse()
			return nil
		}
		if hasActiveOperation(codespace) || codespace.Status != codespace_model.StatusRunning {
			response = idleStopNotApplicableResponse()
			return nil
		}

		settings := effectiveRuntimeSettings(codespace)
		if !settings.AutoStopEnabled || settings.IdleTimeoutSeconds != opts.ObservedIdleTimeoutSeconds || settings.InteractionGeneration != opts.ObservedInteractionGeneration {
			response = &codespacev1.RequestIdleStopResponse{
				Outcome: &codespacev1.RequestIdleStopResponse_ObservationChanged{
					ObservationChanged: &codespacev1.IdleStopObservationChanged{RuntimeSettings: runtimeSettingsMessage(settings)},
				},
			}
			return nil
		}
		expected := snapshotCodespaceState(codespace)
		nextVersion, err := codespace_model.NextVersion(codespace.OperationRVersion)
		if err != nil {
			return ErrRequestIdleStopVersionExhausted
		}
		now := time.Now().Unix()
		codespace.OperationRVersion = nextVersion
		codespace.OperationTrigger = codespace_model.OperationTriggerIdle
		codespace.OperationCreatedUnix = now
		codespace.OperationStartedUnix = 0
		codespace.OperationDeadlineUnix = 0
		codespace.UpdatedUnix = now
		if err := updateCodespaceIfCurrent(ctx, expected, codespace,
			"operation_r_version",
			"operation_trigger",
			"operation_created_unix",
			"operation_started_unix",
			"operation_deadline_unix",
			"updated_unix",
		); err != nil {
			if errors.Is(err, errCodespaceStateChanged) {
				response = idleStopNotApplicableResponse()
				return nil
			}
			return err
		}
		response = idleStopPendingResponse()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func isQueuedIdleStop(codespace *codespace_model.Codespace) bool {
	return codespace_model.ActiveOperationType(codespace) == codespace_model.OperationStop &&
		codespace_model.IsOperationQueued(codespace) &&
		codespace.OperationTrigger == codespace_model.OperationTriggerIdle
}

func idleStopPendingResponse() *codespacev1.RequestIdleStopResponse {
	return &codespacev1.RequestIdleStopResponse{
		Outcome: &codespacev1.RequestIdleStopResponse_Pending{
			Pending: &codespacev1.IdleStopPending{},
		},
	}
}

func idleStopNotApplicableResponse() *codespacev1.RequestIdleStopResponse {
	return &codespacev1.RequestIdleStopResponse{
		Outcome: &codespacev1.RequestIdleStopResponse_NotApplicable{
			NotApplicable: &codespacev1.IdleStopNotApplicable{},
		},
	}
}
