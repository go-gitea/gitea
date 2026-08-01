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
	"gitea.dev/modules/globallock"
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
	CodespaceUUID                 string
	ObservedAutoStopEnabled       bool
	ObservedIdleTimeoutSeconds    int64
	ObservedInteractionGeneration int64
}

// RequestIdleStop creates or confirms a queued idle stop after current setting validation.
func RequestIdleStop(ctx context.Context, manager *codespace_model.Manager, opts RequestIdleStopOptions) (*codespacev1.RequestIdleStopResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if err := validateObservedRuntimeSettings(opts); err != nil {
		return nil, err
	}

	var response *codespacev1.RequestIdleStopResponse
	err := globallock.LockAndDo(ctx, codespaceStateLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentManager, err := loadCodespaceManager(ctx, manager.ID)
			if err != nil {
				return err
			}
			if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
				return ErrRequestIdleStopManagerUnavailable
			}
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
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
				response = idleStopPendingResponse(codespace.OperationRVersion)
				return nil
			}
			if hasActiveOperation(codespace) {
				response = notApplicableIdleStop(codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_OPERATION_CONFLICT)
				return nil
			}
			switch codespace.Status {
			case codespace_model.StatusStopped:
				response = notApplicableIdleStop(codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_ALREADY_STOPPED)
				return nil
			case codespace_model.StatusRunning:
			default:
				response = notApplicableIdleStop(codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_STATE_UNAVAILABLE)
				return nil
			}

			settings := effectiveRuntimeSettings(codespace)
			if !settings.AutoStopEnabled || settingsChanged(settings, opts) {
				response = &codespacev1.RequestIdleStopResponse{
					Outcome: &codespacev1.RequestIdleStopResponse_ObservationChanged{
						ObservationChanged: &codespacev1.IdleStopObservationChanged{RuntimeSettings: runtimeSettingsMessage(settings)},
					},
				}
				return nil
			}
			nextVersion, err := codespace_model.NextVersion(codespace.OperationRVersion)
			if err != nil {
				return ErrRequestIdleStopVersionExhausted
			}
			now := time.Now().Unix()
			codespace.OperationRVersion = nextVersion
			codespace.OperationType = codespace_model.OperationStop
			codespace.OperationStatus = codespace_model.OperationStatusQueued
			codespace.OperationTrigger = codespace_model.OperationTriggerIdle
			codespace.OperationCreatedUnix = now
			codespace.OperationStartedUnix = 0
			codespace.OperationDeadlineUnix = 0
			codespace.UpdatedUnix = now
			if _, err := db.GetEngine(ctx).ID(codespace.UUID).Cols(
				"operation_r_version",
				"operation_type",
				"operation_status",
				"operation_trigger",
				"operation_created_unix",
				"operation_started_unix",
				"operation_deadline_unix",
				"updated_unix",
			).Update(codespace); err != nil {
				return err
			}
			response = idleStopPendingResponse(nextVersion)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func validateObservedRuntimeSettings(opts RequestIdleStopOptions) error {
	if opts.ObservedInteractionGeneration < 0 {
		return errors.New("observed_interaction_generation must not be negative")
	}
	if opts.ObservedAutoStopEnabled {
		if opts.ObservedIdleTimeoutSeconds <= 0 {
			return errors.New("observed_idle_timeout_seconds must be positive when auto stop is enabled")
		}
		return nil
	}
	if opts.ObservedIdleTimeoutSeconds != 0 {
		return errors.New("observed_idle_timeout_seconds must be zero when auto stop is disabled")
	}
	return nil
}

func isQueuedIdleStop(codespace *codespace_model.Codespace) bool {
	return codespace.OperationType == codespace_model.OperationStop &&
		codespace.OperationStatus == codespace_model.OperationStatusQueued &&
		codespace.OperationTrigger == codespace_model.OperationTriggerIdle
}

func idleStopPendingResponse(operationRVersion int64) *codespacev1.RequestIdleStopResponse {
	return &codespacev1.RequestIdleStopResponse{
		Outcome: &codespacev1.RequestIdleStopResponse_Pending{
			Pending: &codespacev1.IdleStopPending{OperationRversion: operationRVersion},
		},
	}
}

func notApplicableIdleStop(reason codespacev1.IdleStopNotApplicableReason) *codespacev1.RequestIdleStopResponse {
	return &codespacev1.RequestIdleStopResponse{
		Outcome: &codespacev1.RequestIdleStopResponse_NotApplicable{
			NotApplicable: &codespacev1.IdleStopNotApplicable{Reason: reason},
		},
	}
}

func settingsChanged(settings RuntimeSettings, opts RequestIdleStopOptions) bool {
	return settings.AutoStopEnabled != opts.ObservedAutoStopEnabled ||
		settings.IdleTimeoutSeconds != opts.ObservedIdleTimeoutSeconds ||
		settings.InteractionGeneration != opts.ObservedInteractionGeneration
}

func codespaceStateLockKey(codespaceUUID string) string {
	return "codespace_interaction_" + codespaceUUID
}
