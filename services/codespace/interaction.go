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
	// ErrInteractionNotFound is returned when the Codespace no longer exists.
	ErrInteractionNotFound = errors.New("codespace not found")
	// ErrInteractionPermissionDenied is returned when the user is not the Codespace creator.
	ErrInteractionPermissionDenied = errors.New("codespace permission denied")
	// ErrInteractionStateUnavailable is returned when the lifecycle state cannot accept the request.
	ErrInteractionStateUnavailable = errors.New("codespace state unavailable")
	// ErrInteractionVersionExhausted is returned when interaction_generation cannot advance.
	ErrInteractionVersionExhausted = errors.New("codespace interaction version exhausted")
	// ErrInteractionInvalidArgument is returned when user-submitted interaction options are invalid.
	ErrInteractionInvalidArgument = errors.New("codespace interaction invalid argument")
)

// ContinueCodespaceOptions identifies one creator keep-alive action.
type ContinueCodespaceOptions struct {
	UserID        int64
	CodespaceUUID string
}

// ContinueCodespaceResult contains the new interaction generation.
type ContinueCodespaceResult struct {
	InteractionGeneration int64
}

// UpdateAutoStopOptions contains one creator auto-stop settings update.
type UpdateAutoStopOptions struct {
	UserID               int64
	CodespaceUUID        string
	Mode                 string
	CustomTimeoutSeconds int64
}

// UpdateAutoStopResult contains the persisted and effective runtime settings.
type UpdateAutoStopResult struct {
	Mode                 string
	CustomTimeoutSeconds int64
	RuntimeSettings      RuntimeSettings
}

// ContinueCodespace records creator activity and cancels a queued idle stop.
func ContinueCodespace(ctx context.Context, opts ContinueCodespaceOptions) (*ContinueCodespaceResult, error) {
	if !setting.Codespace.Enabled {
		return nil, ErrInteractionStateUnavailable
	}
	if err := validateCreatorInteractionOptions(opts.UserID, opts.CodespaceUUID); err != nil {
		return nil, err
	}

	var result *ContinueCodespaceResult
	err := globallock.LockAndDo(ctx, codespaceInteractionLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace, err := loadCreatorCodespace(ctx, opts.UserID, opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if codespace.Status != codespace_model.StatusRunning || (hasActiveOperation(codespace) && !isQueuedIdleStop(codespace)) {
				return ErrInteractionStateUnavailable
			}
			now := time.Now().Unix()
			nextGeneration, err := advanceCodespaceInteraction(ctx, codespace, now)
			if err != nil {
				if err == errInteractionVersionExhausted {
					return ErrInteractionVersionExhausted
				}
				return err
			}
			result = &ContinueCodespaceResult{InteractionGeneration: nextGeneration}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateAutoStop saves creator auto-stop settings and cancels stale queued idle stops.
func UpdateAutoStop(ctx context.Context, opts UpdateAutoStopOptions) (*UpdateAutoStopResult, error) {
	mode, customTimeoutSeconds, err := normalizeAutoStopOptions(opts.Mode, opts.CustomTimeoutSeconds)
	if err != nil {
		return nil, err
	}
	if err := validateCreatorInteractionOptions(opts.UserID, opts.CodespaceUUID); err != nil {
		return nil, err
	}

	var result *UpdateAutoStopResult
	err = globallock.LockAndDo(ctx, codespaceInteractionLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace, err := loadCreatorCodespace(ctx, opts.UserID, opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if codespace.Status != codespace_model.StatusRunning && codespace.Status != codespace_model.StatusStopped {
				return ErrInteractionStateUnavailable
			}
			oldSettings := effectiveRuntimeSettings(codespace)
			changed := codespace.AutoStopMode != mode || codespace.AutoStopTimeoutSeconds != customTimeoutSeconds
			codespace.AutoStopMode = mode
			codespace.AutoStopTimeoutSeconds = customTimeoutSeconds
			newSettings := effectiveRuntimeSettings(codespace)

			cols := []string{"auto_stop_mode", "auto_stop_timeout_seconds"}
			if settingsRuntimePolicyChanged(oldSettings, newSettings) && isQueuedIdleStop(codespace) {
				codespace.UpdatedUnix = time.Now().Unix()
				clearActiveOperation(codespace)
				cols = append(cols,
					"operation_type",
					"operation_status",
					"operation_trigger",
					"operation_created_unix",
					"operation_started_unix",
					"operation_deadline_unix",
					"updated_unix",
				)
			}
			if changed || len(cols) > 2 {
				if _, err := db.GetEngine(ctx).ID(codespace.UUID).Cols(cols...).Update(codespace); err != nil {
					return err
				}
			}
			result = &UpdateAutoStopResult{
				Mode:                 codespace.AutoStopMode,
				CustomTimeoutSeconds: codespace.AutoStopTimeoutSeconds,
				RuntimeSettings:      newSettings,
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateCreatorInteractionOptions(userID int64, codespaceUUID string) error {
	if userID <= 0 {
		return errors.New("user_id must be positive")
	}
	return codespace_model.ValidateUUID(codespaceUUID)
}

func loadCreatorCodespace(ctx context.Context, userID int64, codespaceUUID string) (*codespace_model.Codespace, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrInteractionNotFound
	}
	if codespace.UserID != userID {
		return nil, ErrInteractionPermissionDenied
	}
	return codespace, nil
}

func normalizeAutoStopOptions(mode string, customTimeoutSeconds int64) (string, int64, error) {
	switch mode {
	case codespace_model.AutoStopModeDefault, "":
		return codespace_model.AutoStopModeDefault, 0, nil
	case codespace_model.AutoStopModeNever:
		return codespace_model.AutoStopModeNever, 0, nil
	case codespace_model.AutoStopModeCustom:
		minSeconds := int64(setting.Codespace.AutoStopMinTimeout / time.Second)
		maxSeconds := int64(setting.Codespace.AutoStopMaxTimeout / time.Second)
		if customTimeoutSeconds < minSeconds || customTimeoutSeconds > maxSeconds {
			return "", 0, fmt.Errorf("%w: custom timeout must be between %d and %d seconds", ErrInteractionInvalidArgument, minSeconds, maxSeconds)
		}
		return mode, customTimeoutSeconds, nil
	default:
		return "", 0, fmt.Errorf("%w: invalid auto-stop mode", ErrInteractionInvalidArgument)
	}
}

func settingsRuntimePolicyChanged(oldSettings, newSettings RuntimeSettings) bool {
	return oldSettings.AutoStopEnabled != newSettings.AutoStopEnabled ||
		oldSettings.IdleTimeoutSeconds != newSettings.IdleTimeoutSeconds
}
