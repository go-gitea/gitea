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
	CodespaceID int64
	ManagerID   int64
	Unassigned  bool
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
	ID                  int64
	DisplayStatus       string
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

type governanceViewCache struct {
	users        map[int64]*user_model.User
	repositories map[int64]*repo_model.Repository
	managers     map[int64]*codespace_model.Manager
}

// ListGovernanceCodespaces returns one scoped page without exposing creator-only runtime access data.
func ListGovernanceCodespaces(ctx context.Context, opts GovernanceListOptions) (*GovernanceList, error) {
	if opts.Page <= 0 || opts.PageSize <= 0 || opts.UserID < 0 ||
		(opts.Unassigned && opts.ManagerID != 0) || (!opts.Unassigned && opts.ManagerID <= 0) {
		return nil, errors.New("invalid Codespace governance list options")
	}
	var queryCondition builder.Cond
	if opts.Unassigned {
		queryCondition = builder.Or(
			builder.Eq{"manager_id": 0},
			builder.NotIn("manager_id", builder.Select("id").From("codespace_manager")),
		)
	} else {
		queryCondition = builder.Eq{"manager_id": opts.ManagerID}
		if opts.UserID > 0 {
			queryCondition = builder.And(queryCondition, builder.Eq{"user_id": opts.UserID})
		}
	}
	var rows []*codespace_model.Codespace
	total, err := db.GetEngine(ctx).
		Where(queryCondition).
		Desc("updated_unix", "created_unix", "id").
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Select(codespaceStateColumns).
		FindAndCount(&rows)
	if err != nil {
		return nil, err
	}
	cache, err := loadGovernanceViewCache(ctx, rows)
	if err != nil {
		return nil, err
	}
	result := &GovernanceList{Rows: make([]*GovernanceView, 0, len(rows)), Total: total}
	for _, row := range rows {
		result.Rows = append(result.Rows, governanceCodespaceView(row, cache))
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
	return db.WithTx(ctx, func(ctx context.Context) error {
		codespace, _, err := loadGovernanceTarget(ctx, opts)
		if err != nil {
			return err
		}
		return deleteCodespaceRowForFinal(ctx, codespace)
	})
}

func governanceCodespaceView(codespace *codespace_model.Codespace, cache *governanceViewCache) *GovernanceView {
	manager, managerFound := cache.managers[codespace.ManagerID]
	view := &CreatorCodespaceView{
		ID:          codespace.ID,
		Status:      codespace.Status,
		UpdatedUnix: codespace.UpdatedUnix,
	}
	applyCreatorDisplayState(codespace, view, manager, false)

	result := &GovernanceView{
		ID:            codespace.ID,
		DisplayStatus: view.DisplayStatus,
		UpdatedUnix:   codespace.UpdatedUnix,
		UserID:        codespace.UserID,
		RepoID:        codespace.RepoID,
		RefName:       codespace.RefName,
		ManagerID:     codespace.ManagerID,
	}
	if user := cache.users[codespace.UserID]; user != nil {
		result.UserDisplayName = user.DisplayName()
	}
	if repo := cache.repositories[codespace.RepoID]; repo != nil {
		result.RepoFullName = repo.FullName()
	}
	applyGovernanceManagerFields(result, manager)
	applyGovernanceActions(result, managerFound)
	return result
}

func loadGovernanceViewCache(ctx context.Context, codespaces []*codespace_model.Codespace) (*governanceViewCache, error) {
	viewCache, err := loadCodespaceViewCache(ctx, codespaces)
	if err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(codespaces))
	seenUsers := make(map[int64]struct{}, len(codespaces))
	for _, codespace := range codespaces {
		if codespace.UserID > 0 {
			if _, seen := seenUsers[codespace.UserID]; !seen {
				userIDs = append(userIDs, codespace.UserID)
				seenUsers[codespace.UserID] = struct{}{}
			}
		}
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	return &governanceViewCache{
		users:        users,
		repositories: viewCache.repositories,
		managers:     viewCache.managers,
	}, nil
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

func applyGovernanceActions(view *GovernanceView, managerFound bool) {
	if view.ManagerID > 0 && !managerFound {
		view.CanForceDelete = true
		return
	}
	switch view.DisplayStatus {
	case DisplayRunning, DisplayRecovering, DisplayMetadataRebuilding:
		view.CanStop = true
		view.CanDelete = true
	case DisplayQueued, DisplayBooting, DisplayStopping, DisplayStopped, DisplayResuming, DisplayFailed:
		view.CanDelete = true
	}
	view.CanForceDelete = true
}

func applyGovernanceLifecycleAction(ctx context.Context, opts GovernanceActionOptions, operationType string) (*LifecycleActionResult, error) {
	var result *LifecycleActionResult
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace, manager, err := loadGovernanceTarget(ctx, opts)
		if err != nil {
			return err
		}
		view := &CreatorCodespaceView{}
		applyCreatorDisplayState(codespace, view, manager, false)
		governanceView := &GovernanceView{ManagerID: codespace.ManagerID, DisplayStatus: view.DisplayStatus}
		applyGovernanceActions(governanceView, manager != nil)
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
	return result, err
}

func loadGovernanceTarget(ctx context.Context, opts GovernanceActionOptions) (*codespace_model.Codespace, *codespace_model.Manager, error) {
	if opts.CodespaceID <= 0 || (opts.Unassigned && opts.ManagerID != 0) || (!opts.Unassigned && opts.ManagerID <= 0) {
		return nil, nil, errors.New("invalid Codespace governance target")
	}
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Omit("secrets_encrypted").Get(codespace)
	if err != nil {
		return nil, nil, err
	}
	if !has || (!opts.Unassigned && codespace.ManagerID != opts.ManagerID) {
		return nil, nil, ErrGovernanceNotFound
	}
	var manager *codespace_model.Manager
	if codespace.ManagerID > 0 {
		manager = new(codespace_model.Manager)
		has, err = db.GetEngine(ctx).ID(codespace.ManagerID).Get(manager)
		if err != nil {
			return nil, nil, err
		}
		if !has {
			manager = nil
		}
	}
	if opts.Unassigned && manager != nil {
		return nil, nil, ErrGovernanceNotFound
	}
	return codespace, manager, nil
}
