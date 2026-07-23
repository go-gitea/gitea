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
	// ErrRequestIdleStopNotFound is returned when the Codespace no longer exists.
	ErrRequestIdleStopNotFound = errors.New("codespace not found")
	// ErrRequestIdleStopManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrRequestIdleStopManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrRequestIdleStopManagerUnavailable is returned when the authenticated Manager is not usable.
	ErrRequestIdleStopManagerUnavailable = errors.New("manager is not online")
	// ErrRequestIdleStopVersionExhausted is returned when operation_rversion cannot advance.
	ErrRequestIdleStopVersionExhausted = errors.New("codespace operation version exhausted")
)

// IdleStopOutcome identifies the RequestIdleStop result branch.
type IdleStopOutcome string

const (
	// IdleStopOutcomePending means a queued idle stop exists.
	IdleStopOutcomePending IdleStopOutcome = "pending"
	// IdleStopOutcomeObservationChanged means Manager must refresh its effective runtime settings.
	IdleStopOutcomeObservationChanged IdleStopOutcome = "observation_changed"
	// IdleStopOutcomeNotApplicable means the Codespace cannot accept an idle stop now.
	IdleStopOutcomeNotApplicable IdleStopOutcome = "not_applicable"
)

const (
	// IdleStopReasonOperationConflict means another active operation exists.
	IdleStopReasonOperationConflict = "operation_conflict"
	// IdleStopReasonAlreadyStopped means the Codespace is already stopped.
	IdleStopReasonAlreadyStopped = "already_stopped"
	// IdleStopReasonStateUnavailable means the lifecycle state cannot stop from idle.
	IdleStopReasonStateUnavailable = "state_unavailable"
)

// RequestIdleStopOptions contains one Manager idle-stop authorization request.
type RequestIdleStopOptions struct {
	CodespaceUUID                 string
	ObservedAutoStopEnabled       bool
	ObservedIdleTimeoutSeconds    int64
	ObservedInteractionGeneration int64
}

// RequestIdleStopResult contains the mutually exclusive idle-stop outcome.
type RequestIdleStopResult struct {
	Outcome             IdleStopOutcome
	OperationRVersion   int64
	RuntimeSettings     RuntimeSettings
	NotApplicableReason string
}

// RequestIdleStop creates or confirms a queued idle stop after current setting validation.
func RequestIdleStop(ctx context.Context, manager *codespace_model.Manager, opts RequestIdleStopOptions) (*RequestIdleStopResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if err := validateObservedRuntimeSettings(opts); err != nil {
		return nil, err
	}

	var result *RequestIdleStopResult
	err := globallock.LockAndDo(ctx, requestIdleStopLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentManager, err := loadFetchManager(ctx, manager.ID)
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
				result = &RequestIdleStopResult{
					Outcome:           IdleStopOutcomePending,
					OperationRVersion: codespace.OperationRVersion,
				}
				return nil
			}
			if hasActiveOperation(codespace) {
				result = notApplicableIdleStop(IdleStopReasonOperationConflict)
				return nil
			}
			switch codespace.Status {
			case codespace_model.StatusStopped:
				result = notApplicableIdleStop(IdleStopReasonAlreadyStopped)
				return nil
			case codespace_model.StatusRunning:
			default:
				result = notApplicableIdleStop(IdleStopReasonStateUnavailable)
				return nil
			}

			settings := effectiveRuntimeSettings(codespace)
			if !settings.AutoStopEnabled || settingsChanged(settings, opts) {
				result = &RequestIdleStopResult{
					Outcome:         IdleStopOutcomeObservationChanged,
					RuntimeSettings: settings,
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
			result = &RequestIdleStopResult{
				Outcome:           IdleStopOutcomePending,
				OperationRVersion: nextVersion,
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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

func notApplicableIdleStop(reason string) *RequestIdleStopResult {
	return &RequestIdleStopResult{
		Outcome:             IdleStopOutcomeNotApplicable,
		NotApplicableReason: reason,
	}
}

func settingsChanged(settings RuntimeSettings, opts RequestIdleStopOptions) bool {
	return settings.AutoStopEnabled != opts.ObservedAutoStopEnabled ||
		settings.IdleTimeoutSeconds != opts.ObservedIdleTimeoutSeconds ||
		settings.InteractionGeneration != opts.ObservedInteractionGeneration
}

func requestIdleStopLockKey(codespaceUUID string) string {
	return codespaceInteractionLockKey(codespaceUUID)
}

func codespaceInteractionLockKey(codespaceUUID string) string {
	return "codespace_interaction_" + codespaceUUID
}
