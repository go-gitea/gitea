// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
)

const (
	// DisplayQueued means a create operation is waiting for a Manager.
	DisplayQueued = "queued"
	// DisplayBooting means a Manager is creating or resuming the runtime.
	DisplayBooting = "booting"
	// DisplayRunning means the runtime is currently interactive.
	DisplayRunning = "running"
	// DisplayStopping means a stop operation is active.
	DisplayStopping = "stopping"
	// DisplayStopped means the runtime is stopped and can be resumed.
	DisplayStopped = "stopped"
	// DisplayResuming means a resume operation is active.
	DisplayResuming = "resuming"
	// DisplayDeleting means a delete operation is active.
	DisplayDeleting = "deleting"
	// DisplayFailed means lifecycle processing reached a failed terminal state.
	DisplayFailed = "failed"
	// DisplayMetadataRebuilding means the runtime is running but ready metadata is absent.
	DisplayMetadataRebuilding = "metadata_rebuilding"
	// DisplayRecovering means Gitea is waiting for a running Manager to become usable again.
	DisplayRecovering = "recovering"
)

const (
	refreshTransitionMilliseconds = 2000
	refreshStableMilliseconds     = 15000
)

var (
	// ErrViewNotFound is returned when the Codespace cannot be found.
	ErrViewNotFound = errors.New("codespace view not found")
	// ErrViewPermissionDenied is returned when the caller is not the creator.
	ErrViewPermissionDenied = errors.New("codespace view permission denied")
)

// CreatorListOptions selects creator-owned Codespaces for a list page.
type CreatorListOptions struct {
	UserID int64
	RepoID int64
}

// CreatorDetailOptions selects one creator-owned Codespace for a detail page.
type CreatorDetailOptions struct {
	UserID        int64
	CodespaceUUID string
}

// CreatorCodespaceList contains rows for a creator list page.
type CreatorCodespaceList struct {
	Rows []*CreatorCodespaceView
}

// CreatorCodespaceView contains the server-authoritative presentation state.
type CreatorCodespaceView struct {
	UUID                   string
	RepoID                 int64
	RepoFullName           string
	RepoLink               string
	RefType                string
	RefName                string
	CommitSHA              string
	Status                 string
	DisplayStatus          string
	StatusSummary          string
	LastActiveUnix         int64
	CreatedUnix            int64
	UpdatedUnix            int64
	AutoStopMode           string
	AutoStopTimeoutSeconds int64
	RefreshAfterMillis     int
	Workspace              *CreatorEndpointView
	Endpoints              []CreatorEndpointView
	CanOpen                bool
	CanContinue            bool
	CanStop                bool
	CanResume              bool
	CanDelete              bool
	CanConfigureAutoStop   bool
}

// CreatorEndpointView contains one current open target shown on the detail page.
type CreatorEndpointView struct {
	EndpointID string
	Label      string
	Public     bool
	OpenPath   string
}

// ListCreatorCodespaces returns creator-owned Codespaces for list pages.
func ListCreatorCodespaces(ctx context.Context, opts CreatorListOptions) (*CreatorCodespaceList, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	query := db.GetEngine(ctx).Where("user_id = ?", opts.UserID)
	if opts.RepoID > 0 {
		query.And("repo_id = ?", opts.RepoID)
	}
	var rows []*codespace_model.Codespace
	if err := query.Desc("updated_unix", "created_unix").Find(&rows); err != nil {
		return nil, err
	}
	result := &CreatorCodespaceList{Rows: make([]*CreatorCodespaceView, 0, len(rows))}
	repoCache := make(map[int64]*repo_model.Repository)
	for _, row := range rows {
		view, err := creatorCodespaceView(ctx, row, repoCache)
		if err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, view)
	}
	return result, nil
}

// GetCreatorCodespace returns one creator-owned Codespace for the detail page.
func GetCreatorCodespace(ctx context.Context, opts CreatorDetailOptions) (*CreatorCodespaceView, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrViewNotFound
	}
	if codespace.UserID != opts.UserID {
		return nil, ErrViewPermissionDenied
	}
	return creatorCodespaceView(ctx, codespace, make(map[int64]*repo_model.Repository))
}

func creatorCodespaceView(ctx context.Context, codespace *codespace_model.Codespace, repoCache map[int64]*repo_model.Repository) (*CreatorCodespaceView, error) {
	view := &CreatorCodespaceView{
		UUID:                   codespace.UUID,
		RepoID:                 codespace.RepoID,
		RefType:                codespace.RefType,
		RefName:                codespace.RefName,
		CommitSHA:              codespace.CommitSHA,
		Status:                 codespace.Status,
		LastActiveUnix:         codespace.LastActiveUnix,
		CreatedUnix:            codespace.CreatedUnix,
		UpdatedUnix:            codespace.UpdatedUnix,
		AutoStopMode:           codespace.AutoStopMode,
		AutoStopTimeoutSeconds: codespace.AutoStopTimeoutSeconds,
	}
	if codespace.RepoID > 0 {
		repo, ok, err := loadViewRepository(ctx, repoCache, codespace.RepoID)
		if err != nil {
			return nil, err
		}
		if ok {
			view.RepoFullName = repo.FullName()
			view.RepoLink = repo.Link()
		}
	}
	applyCreatorDisplayState(ctx, codespace, view)
	applyCreatorActions(ctx, codespace, view)
	return view, nil
}

func loadViewRepository(ctx context.Context, repoCache map[int64]*repo_model.Repository, repoID int64) (*repo_model.Repository, bool, error) {
	if repo, ok := repoCache[repoID]; ok {
		return repo, repo != nil, nil
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if repo_model.IsErrRepoNotExist(err) {
		repoCache[repoID] = nil
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, false, err
	}
	repoCache[repoID] = repo
	return repo, true, nil
}

func applyCreatorDisplayState(ctx context.Context, codespace *codespace_model.Codespace, view *CreatorCodespaceView) {
	view.DisplayStatus = stableDisplayStatus(codespace.Status)
	switch codespace.Status {
	case codespace_model.StatusCreating:
		if codespace.OperationStatus == codespace_model.OperationStatusQueued {
			view.DisplayStatus = DisplayQueued
		} else {
			view.DisplayStatus = DisplayBooting
		}
	case codespace_model.StatusRunning:
		view.DisplayStatus = runningDisplayStatus(ctx, codespace, view)
	case codespace_model.StatusStopped:
		if codespace.OperationType == codespace_model.OperationResume {
			view.DisplayStatus = DisplayResuming
		}
	case codespace_model.StatusDeleting:
		view.DisplayStatus = DisplayDeleting
	case codespace_model.StatusFailed:
		view.DisplayStatus = DisplayFailed
	}
	view.RefreshAfterMillis = refreshStableMilliseconds
	if isTransitionDisplayStatus(view.DisplayStatus) {
		view.RefreshAfterMillis = refreshTransitionMilliseconds
	}
	view.StatusSummary = statusSummary(view.DisplayStatus)
}

func stableDisplayStatus(status string) string {
	switch status {
	case codespace_model.StatusRunning:
		return DisplayRunning
	case codespace_model.StatusStopped:
		return DisplayStopped
	case codespace_model.StatusFailed:
		return DisplayFailed
	case codespace_model.StatusDeleting:
		return DisplayDeleting
	default:
		return status
	}
}

func runningDisplayStatus(ctx context.Context, codespace *codespace_model.Codespace, view *CreatorCodespaceView) string {
	if codespace.OperationType == codespace_model.OperationStop && !isQueuedIdleStop(codespace) {
		return DisplayStopping
	}
	managerOnline := false
	if codespace.ManagerID > 0 {
		manager, err := loadFetchManager(ctx, codespace.ManagerID)
		managerOnline = err == nil && manager.RuntimeState == codespace_model.ManagerRuntimeStateOnline && !isManagerOffline(manager)
	}
	if !managerOnline {
		return DisplayRecovering
	}
	entry, hasEntry, err := getRuntimeMetadataEntry(codespace.UUID)
	if err != nil || !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
		return DisplayMetadataRebuilding
	}
	view.Workspace = &CreatorEndpointView{
		EndpointID: "workspace",
		Label:      "Workspace",
		OpenPath:   codespaceDetailPath(codespace.UUID) + "/open",
	}
	for _, endpoint := range entry.Metadata.Endpoints {
		view.Endpoints = append(view.Endpoints, CreatorEndpointView{
			EndpointID: endpoint.EndpointID,
			Label:      endpoint.Label,
			Public:     endpoint.Public,
			OpenPath:   codespaceDetailPath(codespace.UUID) + "/open/" + endpoint.EndpointID,
		})
	}
	return DisplayRunning
}

func applyCreatorActions(ctx context.Context, codespace *codespace_model.Codespace, view *CreatorCodespaceView) {
	view.CanDelete = codespace.Status != codespace_model.StatusDeleting
	view.CanConfigureAutoStop = codespace.Status == codespace_model.StatusRunning || codespace.Status == codespace_model.StatusStopped
	view.CanOpen = view.DisplayStatus == DisplayRunning && view.Workspace != nil && (noActiveOperation(codespace) || isQueuedIdleStop(codespace))
	view.CanContinue = codespace.Status == codespace_model.StatusRunning && isQueuedIdleStop(codespace)
	view.CanStop = codespace.Status == codespace_model.StatusRunning && (noActiveOperation(codespace) || isQueuedIdleStop(codespace))
	view.CanResume = codespace.Status == codespace_model.StatusStopped && noActiveOperation(codespace) && view.DisplayStatus == DisplayStopped && managerOnlineForView(ctx, codespace.ManagerID)
}

func managerOnlineForView(ctx context.Context, managerID int64) bool {
	if managerID <= 0 {
		return false
	}
	manager, err := loadFetchManager(ctx, managerID)
	return err == nil && manager.RuntimeState == codespace_model.ManagerRuntimeStateOnline && !isManagerOffline(manager)
}

func statusSummary(displayStatus string) string {
	switch displayStatus {
	case DisplayQueued:
		return "Waiting for a Codespace Manager"
	case DisplayBooting:
		return "Creating the runtime"
	case DisplayRunning:
		return "Ready"
	case DisplayStopping:
		return "Stopping"
	case DisplayStopped:
		return "Stopped"
	case DisplayResuming:
		return "Resuming"
	case DisplayDeleting:
		return "Deleting"
	case DisplayFailed:
		return "Failed"
	case DisplayMetadataRebuilding:
		return "Runtime metadata is not ready"
	case DisplayRecovering:
		return "Waiting for the Manager"
	default:
		return displayStatus
	}
}

func isTransitionDisplayStatus(displayStatus string) bool {
	switch displayStatus {
	case DisplayQueued, DisplayBooting, DisplayStopping, DisplayResuming, DisplayDeleting, DisplayMetadataRebuilding, DisplayRecovering:
		return true
	default:
		return false
	}
}

func noActiveOperation(codespace *codespace_model.Codespace) bool {
	return !hasActiveOperation(codespace)
}

func codespaceDetailPath(codespaceUUID string) string {
	return "/-/codespaces/" + codespaceUUID
}
