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
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"

	"xorm.io/builder"
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

// GovernanceActionOptions identifies one governance lifecycle request.
type GovernanceActionOptions struct {
	CodespaceUUID string
	ManagerID     int64
	Unassigned    bool
}

// GovernanceListOptions selects one page of Codespaces for a Manager or the unassigned queue.
type GovernanceListOptions struct {
	ManagerID  int64
	UserID     int64
	Unassigned bool
	Page       int
	PageSize   int
}

// GovernanceList contains rows for a governance list page.
type GovernanceList struct {
	Rows  []*GovernanceView
	Total int64
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
	RepoID              int64
	RepoFullName        string
	RefName             string
	ManagerID           int64
	ManagerDisplayName  string
	ManagerRuntimeState string
	CanStop             bool
	CanDelete           bool
	CanForceDelete      bool
}

// ListGovernanceCodespaces returns one scoped page without exposing creator-only runtime access data.
func ListGovernanceCodespaces(ctx context.Context, opts GovernanceListOptions) (*GovernanceList, error) {
	if opts.Page <= 0 || opts.PageSize <= 0 || opts.UserID < 0 || (opts.Unassigned == (opts.ManagerID > 0)) {
		return nil, errors.New("invalid Codespace governance list options")
	}
	condition := builder.Eq{"manager_id": opts.ManagerID}
	if opts.Unassigned {
		condition = nil
	}
	var queryCondition builder.Cond = condition
	if opts.Unassigned {
		queryCondition = builder.Or(
			builder.Eq{"manager_id": 0},
			builder.NotIn("manager_id", builder.Select("id").From("codespace_manager")),
		)
	} else if opts.UserID > 0 {
		queryCondition = builder.And(queryCondition, builder.Eq{"user_id": opts.UserID})
	}
	var rows []*codespace_model.Codespace
	total, err := db.GetEngine(ctx).
		Where(queryCondition).
		Desc("updated_unix", "created_unix").
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		FindAndCount(&rows)
	if err != nil {
		return nil, err
	}
	result := &GovernanceList{Rows: make([]*GovernanceView, 0, len(rows)), Total: total}
	users := make(map[int64]*user_model.User)
	repositories := make(map[int64]*repo_model.Repository)
	managers := make(map[int64]*codespace_model.Manager)
	for _, row := range rows {
		view, err := governanceCodespaceView(ctx, row, users, repositories, managers)
		if err != nil {
			return nil, err
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
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return err
	}
	return globallock.LockAndDo(ctx, codespaceStateLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace, err := loadGovernanceCodespace(ctx, opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if err := validateGovernanceTarget(ctx, codespace, opts); err != nil {
				return err
			}
			return deleteCodespaceForFinal(ctx, opts.CodespaceUUID)
		})
	})
}

func governanceCodespaceView(ctx context.Context, codespace *codespace_model.Codespace, users map[int64]*user_model.User, repositories map[int64]*repo_model.Repository, managers map[int64]*codespace_model.Manager) (*GovernanceView, error) {
	manager, managerFound, err := governanceManager(ctx, managers, codespace.ManagerID)
	if err != nil {
		return nil, err
	}
	view := &CreatorCodespaceView{
		UUID:        codespace.UUID,
		Status:      codespace.Status,
		UpdatedUnix: codespace.UpdatedUnix,
	}
	applyCreatorDisplayState(ctx, codespace, view, manager, false)

	result := &GovernanceView{
		UUID:          codespace.UUID,
		ShortUUID:     shortCodespaceUUID(codespace.UUID),
		DisplayStatus: view.DisplayStatus,
		StatusSummary: view.StatusSummary,
		UpdatedUnix:   codespace.UpdatedUnix,
		UserID:        codespace.UserID,
		RepoID:        codespace.RepoID,
		RefName:       codespace.RefName,
		ManagerID:     codespace.ManagerID,
	}
	if displayName, err := governanceUserDisplayName(ctx, users, codespace.UserID); err != nil {
		return nil, err
	} else {
		result.UserDisplayName = displayName
	}
	if fullName, err := governanceRepositoryFullName(ctx, repositories, codespace.RepoID); err != nil {
		return nil, err
	} else {
		result.RepoFullName = fullName
	}
	if managerFound {
		applyGovernanceManagerFields(result, manager)
	} else {
		applyGovernanceManagerFields(result, nil)
	}
	applyGovernanceActions(result)
	return result, nil
}

func governanceRepositoryFullName(ctx context.Context, cache map[int64]*repo_model.Repository, repoID int64) (string, error) {
	if repoID <= 0 {
		return "", nil
	}
	if repo, ok := cache[repoID]; ok {
		if repo == nil {
			return "", nil
		}
		return repo.FullName(), nil
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if repo_model.IsErrRepoNotExist(err) {
		cache[repoID] = nil
		return "", nil
	}
	if err != nil {
		return "", err
	}
	cache[repoID] = repo
	return repo.FullName(), nil
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

func applyGovernanceActions(view *GovernanceView) {
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
	view.CanForceDelete = true
}

func applyGovernanceLifecycleAction(ctx context.Context, opts GovernanceActionOptions, operationType string) (*LifecycleActionResult, error) {
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}

	var result *LifecycleActionResult
	err := globallock.LockAndDo(ctx, codespaceStateLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace, err := loadGovernanceCodespace(ctx, opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if err := validateGovernanceTarget(ctx, codespace, opts); err != nil {
				return err
			}
			manager, _, err := governanceManager(ctx, make(map[int64]*codespace_model.Manager), codespace.ManagerID)
			if err != nil {
				return err
			}
			view := &CreatorCodespaceView{}
			applyCreatorDisplayState(ctx, codespace, view, manager, false)
			governanceView := &GovernanceView{DisplayStatus: view.DisplayStatus}
			applyGovernanceActions(governanceView)
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

func validateGovernanceTarget(ctx context.Context, codespace *codespace_model.Codespace, opts GovernanceActionOptions) error {
	if opts.ManagerID > 0 {
		if opts.Unassigned || codespace.ManagerID != opts.ManagerID {
			return ErrGovernanceNotFound
		}
		return nil
	}
	if !opts.Unassigned {
		return nil
	}
	_, found, err := governanceManager(ctx, make(map[int64]*codespace_model.Manager), codespace.ManagerID)
	if err != nil {
		return err
	}
	if codespace.ManagerID != 0 && found {
		return ErrGovernanceNotFound
	}
	return nil
}

func loadGovernanceCodespace(ctx context.Context, codespaceUUID string) (*codespace_model.Codespace, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrGovernanceNotFound
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

func shortCodespaceUUID(codespaceUUID string) string {
	if len(codespaceUUID) <= 8 {
		return codespaceUUID
	}
	return codespaceUUID[:8]
}
