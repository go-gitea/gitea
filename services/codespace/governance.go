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
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
)

const (
	// GovernanceScopeSite selects all Codespaces for site administrators.
	GovernanceScopeSite = "site"
	// GovernanceScopeOrganization selects Codespaces bound to Managers owned by one organization.
	GovernanceScopeOrganization = "organization"
)

const (
	managerDisplayPending    = "pending"
	managerDisplayOnline     = "online"
	managerDisplayRecovering = "recovering"
	managerDisplayOffline    = "offline"
)

var (
	// ErrGovernanceNotFound is returned when the Codespace is outside the governance scope.
	ErrGovernanceNotFound = errors.New("codespace governance target not found")
	// ErrGovernanceStateUnavailable is returned when the requested governance action does not apply.
	ErrGovernanceStateUnavailable = errors.New("codespace governance state unavailable")
)

// GovernanceListOptions selects Codespaces for a governance list page.
type GovernanceListOptions struct {
	Scope   string
	OwnerID int64
}

// GovernanceActionOptions identifies one governance lifecycle request.
type GovernanceActionOptions struct {
	Scope         string
	OwnerID       int64
	CodespaceUUID string
}

// GovernanceList contains rows for a governance list page.
type GovernanceList struct {
	Rows []*GovernanceView
}

// GovernanceView contains only the fields non-creator governance pages may show.
type GovernanceView struct {
	UUID                string
	ShortUUID           string
	DisplayStatus       string
	StatusSummary       string
	UpdatedUnix         int64
	UserID              int64
	UserDisplayName     string
	ManagerID           int64
	ManagerDisplayName  string
	ManagerRuntimeState string
	CanStop             bool
	CanDelete           bool
	CanForceDelete      bool
}

// ListGovernanceCodespaces returns site or organization governance rows.
func ListGovernanceCodespaces(ctx context.Context, opts GovernanceListOptions) (*GovernanceList, error) {
	if err := validateGovernanceScope(opts.Scope, opts.OwnerID); err != nil {
		return nil, err
	}

	rows, err := listGovernanceModels(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := &GovernanceList{Rows: make([]*GovernanceView, 0, len(rows))}
	users := make(map[int64]*user_model.User)
	managers := make(map[int64]*codespace_model.Manager)
	for _, row := range rows {
		view, err := governanceCodespaceView(ctx, row, opts.Scope, users, managers)
		if err != nil {
			return nil, err
		}
		if opts.Scope == GovernanceScopeOrganization && view.DisplayStatus == DisplayQueued {
			continue
		}
		result.Rows = append(result.Rows, view)
	}
	return result, nil
}

// StopGovernanceCodespace queues a governance stop operation.
func StopGovernanceCodespace(ctx context.Context, opts GovernanceActionOptions) (*LifecycleActionResult, error) {
	return applyGovernanceLifecycleAction(ctx, opts, codespace_model.OperationStop)
}

// DeleteGovernanceCodespace deletes or queues deletion from a governance list.
func DeleteGovernanceCodespace(ctx context.Context, opts GovernanceActionOptions) (*LifecycleActionResult, error) {
	return applyGovernanceLifecycleAction(ctx, opts, codespace_model.OperationDelete)
}

// ForceDeleteCodespace physically deletes one Codespace from the site governance list.
func ForceDeleteCodespace(ctx context.Context, opts GovernanceActionOptions) error {
	if opts.Scope != GovernanceScopeSite {
		return ErrGovernanceNotFound
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return err
	}
	return globallock.LockAndDo(ctx, codespaceLifecycleActionLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				return ErrGovernanceNotFound
			}
			return deleteCodespaceForFinal(ctx, opts.CodespaceUUID)
		})
	})
}

func listGovernanceModels(ctx context.Context, opts GovernanceListOptions) ([]*codespace_model.Codespace, error) {
	query := db.GetEngine(ctx)
	if opts.Scope == GovernanceScopeOrganization {
		query = query.Join("INNER", "codespace_manager", "codespace.manager_id = codespace_manager.id").
			Where("codespace.manager_id > 0 AND codespace_manager.owner_id = ?", opts.OwnerID)
	}
	var rows []*codespace_model.Codespace
	if err := query.Desc("codespace.updated_unix", "codespace.created_unix").Find(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func governanceCodespaceView(ctx context.Context, codespace *codespace_model.Codespace, scope string, users map[int64]*user_model.User, managers map[int64]*codespace_model.Manager) (*GovernanceView, error) {
	view := &CreatorCodespaceView{
		UUID:        codespace.UUID,
		Status:      codespace.Status,
		UpdatedUnix: codespace.UpdatedUnix,
	}
	applyCreatorDisplayState(ctx, codespace, view)

	result := &GovernanceView{
		UUID:          codespace.UUID,
		ShortUUID:     shortCodespaceUUID(codespace.UUID),
		DisplayStatus: view.DisplayStatus,
		StatusSummary: view.StatusSummary,
		UpdatedUnix:   codespace.UpdatedUnix,
		UserID:        codespace.UserID,
		ManagerID:     codespace.ManagerID,
	}
	if displayName, err := governanceUserDisplayName(ctx, users, codespace.UserID); err != nil {
		return nil, err
	} else {
		result.UserDisplayName = displayName
	}
	if manager, ok, err := governanceManager(ctx, managers, codespace.ManagerID); err != nil {
		return nil, err
	} else if ok {
		applyGovernanceManagerFields(result, manager)
	} else {
		applyGovernanceManagerFields(result, nil)
	}
	applyGovernanceActions(scope, result)
	return result, nil
}

func applyGovernanceManagerFields(view *GovernanceView, manager *codespace_model.Manager) {
	if manager == nil {
		view.ManagerRuntimeState = managerDisplayPending
		return
	}
	view.ManagerDisplayName = manager.Name
	if view.ManagerDisplayName == "" {
		view.ManagerDisplayName = fmt.Sprintf("Manager %d", manager.ID)
	}
	switch {
	case manager.RuntimeState == codespace_model.ManagerRuntimeStateOnline && !isManagerOffline(manager):
		view.ManagerRuntimeState = managerDisplayOnline
	case manager.RuntimeState == codespace_model.ManagerRuntimeStateRecovering && !isManagerOffline(manager):
		view.ManagerRuntimeState = managerDisplayRecovering
	default:
		view.ManagerRuntimeState = managerDisplayOffline
	}
}

func applyGovernanceActions(scope string, view *GovernanceView) {
	switch view.DisplayStatus {
	case DisplayRunning, DisplayRecovering, DisplayMetadataRebuilding:
		view.CanStop = true
		view.CanDelete = true
	case DisplayQueued, DisplayBooting, DisplayStopping, DisplayStopped, DisplayResuming, DisplayFailed:
		view.CanDelete = true
	}
	if view.DisplayStatus == DisplayDeleting {
		view.CanDelete = false
	}
	view.CanForceDelete = scope == GovernanceScopeSite
}

func applyGovernanceLifecycleAction(ctx context.Context, opts GovernanceActionOptions, operationType string) (*LifecycleActionResult, error) {
	if err := validateGovernanceScope(opts.Scope, opts.OwnerID); err != nil {
		return nil, err
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}

	var result *LifecycleActionResult
	err := globallock.LockAndDo(ctx, codespaceLifecycleActionLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace, err := loadGovernanceCodespace(ctx, opts)
			if err != nil {
				return err
			}
			view := &CreatorCodespaceView{}
			applyCreatorDisplayState(ctx, codespace, view)
			governanceView := &GovernanceView{DisplayStatus: view.DisplayStatus}
			applyGovernanceActions(opts.Scope, governanceView)
			switch operationType {
			case codespace_model.OperationStop:
				if !governanceView.CanStop {
					return ErrGovernanceStateUnavailable
				}
				result, err = applyStopAction(ctx, codespace, time.Now().Unix())
			case codespace_model.OperationDelete:
				if !governanceView.CanDelete {
					return ErrGovernanceStateUnavailable
				}
				result, err = applyDeleteAction(ctx, codespace, time.Now().Unix())
			default:
				err = fmt.Errorf("unsupported governance operation %q", operationType)
			}
			if errors.Is(err, ErrLifecycleActionStateUnavailable) {
				return ErrGovernanceStateUnavailable
			}
			return err
		})
	})
	return result, err
}

func loadGovernanceCodespace(ctx context.Context, opts GovernanceActionOptions) (*codespace_model.Codespace, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrGovernanceNotFound
	}
	if opts.Scope == GovernanceScopeOrganization {
		manager, ok, err := governanceManager(ctx, map[int64]*codespace_model.Manager{}, codespace.ManagerID)
		if err != nil || !ok || manager.OwnerID != opts.OwnerID {
			return nil, ErrGovernanceNotFound
		}
	}
	return codespace, nil
}

func governanceUserDisplayName(ctx context.Context, cache map[int64]*user_model.User, userID int64) (string, error) {
	if userID <= 0 {
		return "", nil
	}
	if user, ok := cache[userID]; ok {
		if user == nil {
			return "", nil
		}
		return user.DisplayName(), nil
	}
	user, err := user_model.GetUserByID(ctx, userID)
	if user_model.IsErrUserNotExist(err) {
		cache[userID] = nil
		return "", nil
	}
	if err != nil {
		return "", err
	}
	cache[userID] = user
	return user.DisplayName(), nil
}

func governanceManager(ctx context.Context, cache map[int64]*codespace_model.Manager, managerID int64) (*codespace_model.Manager, bool, error) {
	if managerID <= 0 {
		return nil, false, nil
	}
	if manager, ok := cache[managerID]; ok {
		return manager, manager != nil, nil
	}
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil {
		return nil, false, err
	}
	if !has {
		cache[managerID] = nil
		return nil, false, nil
	}
	cache[managerID] = manager
	return manager, true, nil
}

func validateGovernanceScope(scope string, ownerID int64) error {
	switch scope {
	case GovernanceScopeSite:
		return nil
	case GovernanceScopeOrganization:
		if ownerID <= 0 {
			return errors.New("owner_id must be positive")
		}
		return nil
	default:
		return fmt.Errorf("unsupported governance scope %q", scope)
	}
}

func shortCodespaceUUID(codespaceUUID string) string {
	if len(codespaceUUID) <= 8 {
		return codespaceUUID
	}
	return codespaceUUID[:8]
}
