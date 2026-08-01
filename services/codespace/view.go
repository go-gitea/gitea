// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
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

	// DetailModeOverview shows stable runtime access and configuration.
	DetailModeOverview = "overview"
	// DetailModeLogs shows lifecycle progress and operation output.
	DetailModeLogs = "logs"
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
	UserID      int64
	RepoID      int64
	RepoOwnerID int64
	RefType     string
	RefName     string
	CommitSHA   string
	Page        int
	PageSize    int
	Limit       int
}

// CreatorDetailOptions selects one creator-owned Codespace for a detail page.
type CreatorDetailOptions struct {
	UserID        int64
	CodespaceUUID string
}

// CreatorCodespaceList contains rows for a creator list page.
type CreatorCodespaceList struct {
	Rows  []*CreatorCodespaceView
	Total int64
}

// CreatorCodespaceView contains the server-authoritative presentation state.
type CreatorCodespaceView struct {
	UUID                 string
	ShortUUID            string
	RepoID               int64
	RepoFullName         string
	RepoLink             string
	CommitLink           string
	RefType              string
	RefName              string
	RefDisplayName       string
	CommitSHA            string
	EnvironmentTag       string
	Status               string
	DisplayStatus        string
	DetailMode           string
	BootStageKey         string
	DisplayStatusKey     string
	StatusSummary        string
	StatusSummaryKey     string
	LastActiveUnix       int64
	CreatedUnix          int64
	UpdatedUnix          int64
	AutoStop             CreatorAutoStopView
	RefreshAfterMillis   int
	Workspace            *CreatorEndpointView
	Endpoints            []CreatorEndpointView
	SSH                  *CreatorSSHView
	ResourceUsage        *CreatorResourceUsageView
	CanOpen              bool
	CanContinue          bool
	CanStop              bool
	CanResume            bool
	CanDelete            bool
	CanConfigureAutoStop bool
}

// CreatorAutoStopView contains the persisted and effective auto-stop settings shown to the creator.
type CreatorAutoStopView struct {
	Mode                    string
	Timeout                 CreatorDurationView
	Default                 CreatorDurationView
	Minimum                 CreatorDurationView
	Maximum                 CreatorDurationView
	EffectiveEnabled        bool
	EffectiveTimeout        CreatorDurationView
	CustomTimeoutOutOfRange bool
}

// CreatorDurationView contains an exact duration split into a form value and unit.
type CreatorDurationView struct {
	Value          int64
	Unit           string
	TranslationKey string
}

// CreatorEndpointView contains one current open target shown on the detail page.
type CreatorEndpointView struct {
	EndpointID string
	Label      string
	Port       uint16
	Public     bool
	OpenPath   string
	CanOpen    bool
}

// CreatorSSHView contains the current SSH command shown on the detail page.
type CreatorSSHView struct {
	Command            string
	HostKeyAlgorithm   string
	HostKeyFingerprint string
	HostKeyUpdatedUnix int64
}

// CreatorResourceMetricView contains one runtime resource usage measurement.
type CreatorResourceMetricView struct {
	Used         int64
	Limit        int64
	UsedDisplay  string
	LimitDisplay string
}

// CreatorResourceUsageView contains the latest runtime resource usage summary.
type CreatorResourceUsageView struct {
	CPU          CreatorResourceMetricView
	Memory       CreatorResourceMetricView
	Disk         CreatorResourceMetricView
	ObservedUnix int64
}

type creatorViewCache struct {
	repositories map[int64]*repo_model.Repository
	managers     map[int64]*codespace_model.Manager
}

// ListCreatorCodespaces returns creator-owned Codespaces for list pages.
func ListCreatorCodespaces(ctx context.Context, opts CreatorListOptions) (*CreatorCodespaceList, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	query := db.GetEngine(ctx).Table("codespace").Select("codespace.*").Where("codespace.user_id = ?", opts.UserID)
	if opts.RepoID > 0 {
		query.And("codespace.repo_id = ?", opts.RepoID)
	}
	if opts.RepoOwnerID > 0 {
		query.Join("INNER", "repository", "repository.id = codespace.repo_id AND repository.owner_id = ?", opts.RepoOwnerID)
	}
	if opts.RefType != "" || opts.RefName != "" {
		if opts.RefType == "" || opts.RefName == "" {
			return nil, errors.New("ref_type and ref_name must be provided together")
		}
		if opts.CommitSHA != "" {
			return nil, errors.New("commit_sha cannot be combined with ref_type and ref_name")
		}
		query.And("codespace.ref_type = ? AND codespace.ref_name = ?", opts.RefType, opts.RefName)
	} else if opts.CommitSHA != "" {
		query.And("codespace.commit_sha = ?", opts.CommitSHA)
	}
	if opts.PageSize > 0 {
		if opts.Page <= 0 {
			opts.Page = 1
		}
		query.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	} else if opts.Limit > 0 {
		query.Limit(opts.Limit)
	}
	var rows []*codespace_model.Codespace
	query.Desc("codespace.updated_unix", "codespace.created_unix", "codespace.uuid")
	var total int64
	if opts.PageSize > 0 {
		var err error
		total, err = query.FindAndCount(&rows)
		if err != nil {
			return nil, err
		}
	} else if err := query.Find(&rows); err != nil {
		return nil, err
	}
	cache, err := loadCreatorViewCache(ctx, rows)
	if err != nil {
		return nil, err
	}
	result := &CreatorCodespaceList{Rows: make([]*CreatorCodespaceView, 0, len(rows)), Total: total}
	for _, row := range rows {
		view, err := creatorCodespaceView(ctx, row, cache, false)
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
	cache, err := loadCreatorViewCache(ctx, []*codespace_model.Codespace{codespace})
	if err != nil {
		return nil, err
	}
	return creatorCodespaceView(ctx, codespace, cache, true)
}

func creatorCodespaceView(ctx context.Context, codespace *codespace_model.Codespace, cache *creatorViewCache, includeDetailData bool) (*CreatorCodespaceView, error) {
	refDisplayName := codespace.RefName
	if codespace.RefType == "pull" {
		if index, err := pullIndexFromCanonicalRef(codespace.RefName); err == nil {
			refDisplayName = fmt.Sprintf("#%d", index)
		}
	}
	view := &CreatorCodespaceView{
		UUID:           codespace.UUID,
		ShortUUID:      shortCodespaceUUID(codespace.UUID),
		RepoID:         codespace.RepoID,
		RefType:        codespace.RefType,
		RefName:        codespace.RefName,
		RefDisplayName: refDisplayName,
		CommitSHA:      codespace.CommitSHA,
		EnvironmentTag: codespace.EnvironmentTag,
		Status:         codespace.Status,
		LastActiveUnix: codespace.LastActiveUnix,
		CreatedUnix:    codespace.CreatedUnix,
		UpdatedUnix:    codespace.UpdatedUnix,
		AutoStop:       creatorAutoStopView(codespace),
	}
	if codespace.RepoID > 0 {
		if repo := cache.repositories[codespace.RepoID]; repo != nil {
			view.RepoFullName = repo.FullName()
			view.RepoLink = repo.Link()
			view.CommitLink = repo.CommitLink(codespace.CommitSHA)
		}
	}
	manager := cache.managers[codespace.ManagerID]
	applyCreatorDisplayState(ctx, codespace, view, manager, includeDetailData)
	switch view.DisplayStatus {
	case DisplayRunning, DisplayStopped, DisplayRecovering, DisplayMetadataRebuilding:
		view.DetailMode = DetailModeOverview
	default:
		view.DetailMode = DetailModeLogs
	}
	if includeDetailData && (view.DisplayStatus == DisplayBooting || view.DisplayStatus == DisplayResuming) {
		entry, hasEntry, err := getRuntimeMetadataEntry(codespace.UUID)
		if err == nil && hasEntry && entry.Metadata.Boot.OperationRVersion == codespace.OperationRVersion {
			switch entry.Metadata.Boot.Stage {
			case bootStagePrepareRuntime, bootStageInitializeSystem, bootStagePrepareWorkspace, bootStageStartEnvironment, bootStagePublishReady, bootStageReady:
				view.BootStageKey = "codespace.boot_stage." + strings.ReplaceAll(entry.Metadata.Boot.Stage, "-", "_")
			}
		}
	}
	applyCreatorActions(codespace, view, manager)
	return view, nil
}

func loadCreatorViewCache(ctx context.Context, codespaces []*codespace_model.Codespace) (*creatorViewCache, error) {
	repoIDs := make([]int64, 0, len(codespaces))
	managerIDs := make([]int64, 0, len(codespaces))
	seenRepos := make(map[int64]struct{}, len(codespaces))
	seenManagers := make(map[int64]struct{}, len(codespaces))
	for _, codespace := range codespaces {
		if codespace.RepoID > 0 {
			if _, seen := seenRepos[codespace.RepoID]; !seen {
				repoIDs = append(repoIDs, codespace.RepoID)
				seenRepos[codespace.RepoID] = struct{}{}
			}
		}
		if codespace.ManagerID > 0 {
			if _, seen := seenManagers[codespace.ManagerID]; !seen {
				managerIDs = append(managerIDs, codespace.ManagerID)
				seenManagers[codespace.ManagerID] = struct{}{}
			}
		}
	}

	repositories, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return nil, err
	}
	managers := make(map[int64]*codespace_model.Manager, len(managerIDs))
	if len(managerIDs) > 0 {
		var rows []*codespace_model.Manager
		if err := db.GetEngine(ctx).In("id", managerIDs).Find(&rows); err != nil {
			return nil, err
		}
		for _, manager := range rows {
			managers[manager.ID] = manager
		}
	}
	return &creatorViewCache{repositories: repositories, managers: managers}, nil
}

func creatorAutoStopView(codespace *codespace_model.Codespace) CreatorAutoStopView {
	defaultSeconds := int64(setting.Codespace.AutoStopDefaultTimeout / time.Second)
	minimumSeconds := int64(setting.Codespace.AutoStopMinTimeout / time.Second)
	maximumSeconds := int64(setting.Codespace.AutoStopMaxTimeout / time.Second)
	timeoutSeconds := defaultSeconds
	if codespace.AutoStopMode == codespace_model.AutoStopModeCustom {
		timeoutSeconds = codespace.AutoStopTimeoutSeconds
	}
	effective := effectiveRuntimeSettings(codespace)
	return CreatorAutoStopView{
		Mode:                    codespace.AutoStopMode,
		Timeout:                 creatorDurationView(timeoutSeconds),
		Default:                 creatorDurationView(defaultSeconds),
		Minimum:                 creatorDurationView(minimumSeconds),
		Maximum:                 creatorDurationView(maximumSeconds),
		EffectiveEnabled:        effective.AutoStopEnabled,
		EffectiveTimeout:        creatorDurationView(effective.IdleTimeoutSeconds),
		CustomTimeoutOutOfRange: codespace.AutoStopMode == codespace_model.AutoStopModeCustom && (codespace.AutoStopTimeoutSeconds < minimumSeconds || codespace.AutoStopTimeoutSeconds > maximumSeconds),
	}
}

func creatorDurationView(seconds int64) CreatorDurationView {
	units := []struct {
		name           string
		translationKey string
		seconds        int64
	}{
		{"days", "tool.days", 24 * 60 * 60},
		{"hours", "tool.hours", 60 * 60},
		{"minutes", "tool.minutes", 60},
	}
	for _, unit := range units {
		if seconds > 0 && seconds%unit.seconds == 0 {
			return CreatorDurationView{Value: seconds / unit.seconds, Unit: unit.name, TranslationKey: unit.translationKey}
		}
	}
	return CreatorDurationView{Value: seconds, Unit: "seconds", TranslationKey: "tool.seconds"}
}

func applyCreatorDisplayState(ctx context.Context, codespace *codespace_model.Codespace, view *CreatorCodespaceView, manager *codespace_model.Manager, includeDetailData bool) {
	view.DisplayStatus = codespace.Status
	switch codespace.Status {
	case codespace_model.StatusCreating:
		if codespace.OperationStatus == codespace_model.OperationStatusQueued {
			view.DisplayStatus = DisplayQueued
		} else {
			view.DisplayStatus = DisplayBooting
		}
	case codespace_model.StatusRunning:
		view.DisplayStatus = runningDisplayStatus(ctx, codespace, view, manager, includeDetailData)
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
	switch view.DisplayStatus {
	case DisplayQueued, DisplayBooting, DisplayStopping, DisplayResuming, DisplayDeleting, DisplayMetadataRebuilding, DisplayRecovering:
		view.RefreshAfterMillis = refreshTransitionMilliseconds
	}
	view.StatusSummary = statusSummary(view.DisplayStatus)
	view.DisplayStatusKey = "codespace.status." + view.DisplayStatus
	view.StatusSummaryKey = "codespace.status_summary." + view.DisplayStatus
}

func runningDisplayStatus(ctx context.Context, codespace *codespace_model.Codespace, view *CreatorCodespaceView, manager *codespace_model.Manager, includeDetailData bool) string {
	if codespace.OperationType == codespace_model.OperationStop && !isQueuedIdleStop(codespace) {
		return DisplayStopping
	}
	if manager == nil || manager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(manager) {
		return DisplayRecovering
	}
	entry, hasEntry, err := getRuntimeMetadataEntry(codespace.UUID)
	if err != nil || !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
		return DisplayMetadataRebuilding
	}
	for _, endpoint := range entry.Metadata.Endpoints {
		if endpoint.EndpointID == workspaceEndpointID {
			view.Workspace = &CreatorEndpointView{
				EndpointID: workspaceEndpointID,
				Label:      workspaceEndpointLabel,
				OpenPath:   codespaceDetailPath(codespace.UUID) + "/open",
			}
			continue
		}
		if !includeDetailData {
			continue
		}
		portText, hasPortPrefix := strings.CutPrefix(endpoint.EndpointID, "port-")
		var port uint64
		if hasPortPrefix {
			port, _ = strconv.ParseUint(portText, 10, 16)
		}
		view.Endpoints = append(view.Endpoints, CreatorEndpointView{
			EndpointID: endpoint.EndpointID,
			Label:      endpoint.Label,
			Port:       uint16(port),
			Public:     endpoint.Public,
			OpenPath:   codespaceDetailPath(codespace.UUID) + "/open/" + endpoint.EndpointID,
		})
	}
	if view.Workspace == nil {
		return DisplayMetadataRebuilding
	}
	if includeDetailData {
		view.ResourceUsage = creatorResourceUsageView(entry.Metadata.ResourceUsage)
		view.SSH = creatorSSHView(ctx, codespace, manager)
	}
	return DisplayRunning
}

func creatorSSHView(ctx context.Context, codespace *codespace_model.Codespace, manager *codespace_model.Manager) *CreatorSSHView {
	address := new(codespace_model.ManagerAddress)
	has, err := db.GetEngine(ctx).
		Where("manager_id = ? AND kind = ?", codespace.ManagerID, codespace_model.ManagerAddressSSH).
		Get(address)
	if err != nil || !has || address.Address == "" {
		return nil
	}
	host, port, err := net.SplitHostPort(address.Address)
	if err != nil {
		return nil
	}
	command := "ssh cs-" + codespace.UUID + "@" + host
	if port != "" && port != "22" {
		command = "ssh -p " + port + " cs-" + codespace.UUID + "@" + host
	}
	return &CreatorSSHView{
		Command:            command,
		HostKeyAlgorithm:   strings.TrimSpace(manager.GatewaySSHHostKeyAlgorithm),
		HostKeyFingerprint: strings.TrimSpace(manager.GatewaySSHHostKeyFingerprintSHA256),
		HostKeyUpdatedUnix: manager.GatewaySSHHostKeyUpdatedUnix,
	}
}

func creatorResourceUsageView(usage runtimeMetadataResourceUsage) *CreatorResourceUsageView {
	return &CreatorResourceUsageView{
		CPU: CreatorResourceMetricView{
			Used:         usage.CPU.UsedMillicores,
			Limit:        usage.CPU.LimitMillicores,
			UsedDisplay:  fmt.Sprintf("%dm", usage.CPU.UsedMillicores),
			LimitDisplay: fmt.Sprintf("%dm", usage.CPU.LimitMillicores),
		},
		Memory: CreatorResourceMetricView{
			Used:         usage.Memory.UsedBytes,
			Limit:        usage.Memory.LimitBytes,
			UsedDisplay:  formatBytes(usage.Memory.UsedBytes),
			LimitDisplay: formatBytes(usage.Memory.LimitBytes),
		},
		Disk: CreatorResourceMetricView{
			Used:         usage.Disk.UsedBytes,
			Limit:        usage.Disk.LimitBytes,
			UsedDisplay:  formatBytes(usage.Disk.UsedBytes),
			LimitDisplay: formatBytes(usage.Disk.LimitBytes),
		},
		ObservedUnix: usage.ObservedUnix,
	}
}

func formatBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := int64(unit), 0
	for n := value / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func applyCreatorActions(codespace *codespace_model.Codespace, view *CreatorCodespaceView, manager *codespace_model.Manager) {
	view.CanDelete = codespace.Status != codespace_model.StatusDeleting
	view.CanConfigureAutoStop = codespace.Status == codespace_model.StatusRunning || codespace.Status == codespace_model.StatusStopped
	view.CanOpen = view.DisplayStatus == DisplayRunning && view.Workspace != nil && (!hasActiveOperation(codespace) || isQueuedIdleStop(codespace))
	if view.Workspace != nil {
		view.Workspace.CanOpen = view.CanOpen
	}
	for i := range view.Endpoints {
		view.Endpoints[i].CanOpen = view.DisplayStatus == DisplayRunning && (!hasActiveOperation(codespace) || (!view.Endpoints[i].Public && isQueuedIdleStop(codespace)))
	}
	view.CanContinue = codespace.Status == codespace_model.StatusRunning && isQueuedIdleStop(codespace)
	view.CanStop = codespace.Status == codespace_model.StatusRunning && (!hasActiveOperation(codespace) || isQueuedIdleStop(codespace))
	view.CanResume = codespace.Status == codespace_model.StatusStopped && !hasActiveOperation(codespace) && view.DisplayStatus == DisplayStopped && manager != nil && manager.RuntimeState == codespace_model.ManagerRuntimeStateOnline && !isManagerOffline(manager)
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

func codespaceDetailPath(codespaceUUID string) string {
	return "/-/codespaces/" + codespaceUUID
}
