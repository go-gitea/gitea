// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"

	"xorm.io/builder"
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

var bootStageTranslationKeys = map[codespacev1.RuntimeBootStage]string{
	codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_RUNTIME:   "codespace.boot_stage.prepare_runtime",
	codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_INITIALIZE_SYSTEM: "codespace.boot_stage.initialize_system",
	codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_WORKSPACE: "codespace.boot_stage.prepare_workspace",
	codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_START_ENVIRONMENT: "codespace.boot_stage.start_environment",
	codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY:     "codespace.boot_stage.publish_ready",
	codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY:             "codespace.boot_stage.ready",
}

// ErrViewNotFound is returned when the Codespace cannot be found.
var ErrViewNotFound = errors.New("codespace view not found")

// CreatorListOptions selects creator-owned Codespaces for a list page.
type CreatorListOptions struct {
	UserID      int64
	RepoID      int64
	RepoOwnerID int64
	RefType     string
	RefName     string
	CommitSHA   string
	Keyword     string
	Page        int
	PageSize    int
	Limit       int
}

// CreatorDetailOptions selects one creator-owned Codespace for a detail page.
type CreatorDetailOptions struct {
	UserID      int64
	CodespaceID int64
}

// CreatorCodespaceList contains rows for a creator list page.
type CreatorCodespaceList struct {
	Rows  []*CreatorCodespaceView
	Total int64
}

// CreatorCodespaceView contains the server-authoritative presentation state.
type CreatorCodespaceView struct {
	ID                   int64
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
	StatusSummaryKey     string
	LastActiveUnix       int64
	CreatedUnix          int64
	UpdatedUnix          int64
	LogSize              int64
	OperationRVersion    int64
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
	CanCancelResume      bool
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
	URL        string
	OpenPath   string
	CanOpen    bool
}

// CreatorSSHView contains the current SSH command shown on the detail page.
type CreatorSSHView struct {
	Command            string
	VSCodeURL          template.URL
	ZedURL             template.URL
	HostKeyAlgorithm   string
	HostKeyFingerprint string
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

type codespaceViewCache struct {
	repositories map[int64]*repo_model.Repository
	managers     map[int64]*codespace_model.Manager
}

// ListCreatorCodespaces returns creator-owned Codespaces for list pages.
func ListCreatorCodespaces(ctx context.Context, opts CreatorListOptions) (*CreatorCodespaceList, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	query := db.GetEngine(ctx).Table("codespace").Select(codespaceStateColumns).Where("codespace.user_id = ?", opts.UserID)
	if opts.RepoID > 0 {
		query.And("codespace.repo_id = ?", opts.RepoID)
	}
	keyword := strings.TrimSpace(opts.Keyword)
	if opts.RepoOwnerID > 0 {
		query.Join("INNER", "repository", "repository.id = codespace.repo_id AND repository.owner_id = ?", opts.RepoOwnerID)
	} else if keyword != "" {
		query.Join("LEFT", "repository", "repository.id = codespace.repo_id")
	}
	if keyword != "" {
		query.Join("LEFT", "`user`", "`user`.id = repository.owner_id")
		pattern := "%" + keyword + "%"
		keywordCondition := builder.Or(
			db.BuildCaseInsensitiveLike("repository.lower_name", pattern),
			db.BuildCaseInsensitiveLike("`user`.lower_name", pattern),
			db.BuildCaseInsensitiveLike("codespace.ref_name", pattern),
			db.BuildCaseInsensitiveLike("codespace.environment_tag", pattern),
		)
		if ownerName, repoName, ok := strings.Cut(keyword, "/"); ok && ownerName != "" && repoName != "" {
			keywordCondition = keywordCondition.Or(builder.And(
				db.BuildCaseInsensitiveLike("`user`.lower_name", "%"+ownerName+"%"),
				db.BuildCaseInsensitiveLike("repository.lower_name", "%"+repoName+"%"),
			))
		}
		query.And(keywordCondition)
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
	query.Desc("codespace.updated_unix", "codespace.created_unix", "codespace.id")
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
	cache, err := loadCodespaceViewCache(ctx, rows)
	if err != nil {
		return nil, err
	}
	result := &CreatorCodespaceList{Rows: make([]*CreatorCodespaceView, 0, len(rows)), Total: total}
	for _, row := range rows {
		result.Rows = append(result.Rows, creatorCodespaceView(row, cache, false))
	}
	return result, nil
}

// GetCreatorCodespace returns one creator-owned Codespace for the detail page.
func GetCreatorCodespace(ctx context.Context, opts CreatorDetailOptions) (*CreatorCodespaceView, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	if opts.CodespaceID <= 0 {
		return nil, errors.New("codespace_id must be positive")
	}
	codespace, has, err := findCreatorCodespace(ctx, opts.UserID, opts.CodespaceID)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrViewNotFound
	}
	cache, err := loadCodespaceViewCache(ctx, []*codespace_model.Codespace{codespace})
	if err != nil {
		return nil, err
	}
	return creatorCodespaceView(codespace, cache, true), nil
}

func findCreatorCodespace(ctx context.Context, userID, codespaceID int64) (*codespace_model.Codespace, bool, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceID).Where("user_id = ?", userID).Omit("secrets_encrypted").Get(codespace)
	return codespace, has, err
}

func creatorCodespaceView(codespace *codespace_model.Codespace, cache *codespaceViewCache, includeDetailData bool) *CreatorCodespaceView {
	refDisplayName := codespace.RefName
	if codespace.RefType == "pull" {
		if index, err := pullIndexFromCanonicalRef(codespace.RefName); err == nil {
			refDisplayName = fmt.Sprintf("#%d", index)
		}
	}
	view := &CreatorCodespaceView{
		ID:                codespace.ID,
		RepoID:            codespace.RepoID,
		RefType:           codespace.RefType,
		RefName:           codespace.RefName,
		RefDisplayName:    refDisplayName,
		CommitSHA:         codespace.CommitSHA,
		EnvironmentTag:    codespace.EnvironmentTag,
		Status:            codespace.Status,
		LastActiveUnix:    codespace.LastActiveUnix,
		CreatedUnix:       codespace.CreatedUnix,
		UpdatedUnix:       codespace.UpdatedUnix,
		LogSize:           codespace.LogSize,
		OperationRVersion: codespace.OperationRVersion,
		AutoStop:          creatorAutoStopView(codespace),
	}
	if codespace.RepoID > 0 {
		if repo := cache.repositories[codespace.RepoID]; repo != nil {
			view.RepoFullName = repo.FullName()
			view.RepoLink = repo.Link()
			view.CommitLink = repo.CommitLink(codespace.CommitSHA)
		}
	}
	manager := cache.managers[codespace.ManagerID]
	applyCreatorDisplayState(codespace, view, manager, includeDetailData)
	switch view.DisplayStatus {
	case DisplayRunning, DisplayStopped, DisplayRecovering, DisplayMetadataRebuilding:
		view.DetailMode = DetailModeOverview
	default:
		view.DetailMode = DetailModeLogs
	}
	if includeDetailData && (view.DisplayStatus == DisplayBooting || view.DisplayStatus == DisplayResuming) {
		entry, hasEntry, err := getRuntimeMetadataEntry(codespace.ID)
		if err == nil && hasEntry && entry.Metadata.Boot.OperationRVersion == codespace.OperationRVersion {
			view.BootStageKey = bootStageTranslationKeys[entry.Metadata.Boot.Stage]
		}
	}
	applyCreatorActions(codespace, view, manager)
	return view
}

func loadCodespaceViewCache(ctx context.Context, codespaces []*codespace_model.Codespace) (*codespaceViewCache, error) {
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
	return &codespaceViewCache{repositories: repositories, managers: managers}, nil
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

func applyCreatorDisplayState(codespace *codespace_model.Codespace, view *CreatorCodespaceView, manager *codespace_model.Manager, includeDetailData bool) {
	managerReady := manager != nil && manager.RuntimeState == codespace_model.ManagerRuntimeStateOnline && !isManagerOffline(manager)
	view.DisplayStatus = codespace.Status
	switch codespace.Status {
	case codespace_model.StatusCreating:
		if codespace_model.IsOperationQueued(codespace) {
			view.DisplayStatus = DisplayQueued
		} else {
			view.DisplayStatus = DisplayBooting
		}
	case codespace_model.StatusRunning:
		switch {
		case codespace_model.ActiveOperationType(codespace) == codespace_model.OperationStop && !isQueuedIdleStop(codespace):
			view.DisplayStatus = DisplayStopping
		case !managerReady:
			view.DisplayStatus = DisplayRecovering
		case !applyRuntimeAccessView(codespace, view, manager, includeDetailData):
			view.DisplayStatus = DisplayMetadataRebuilding
		default:
			view.DisplayStatus = DisplayRunning
		}
	case codespace_model.StatusStopped:
		if codespace_model.ActiveOperationType(codespace) == codespace_model.OperationResume {
			view.DisplayStatus = DisplayResuming
		}
	case codespace_model.StatusDeleting:
		view.DisplayStatus = DisplayDeleting
	case codespace_model.StatusFailed:
		view.DisplayStatus = DisplayFailed
	}
	if managerReady && hasActiveStartup(codespace, time.Now().Unix()) {
		applyRuntimeAccessView(codespace, view, manager, includeDetailData)
	}
	view.RefreshAfterMillis = refreshStableMilliseconds
	switch view.DisplayStatus {
	case DisplayQueued, DisplayBooting, DisplayStopping, DisplayResuming, DisplayDeleting, DisplayMetadataRebuilding, DisplayRecovering:
		view.RefreshAfterMillis = refreshTransitionMilliseconds
	}
	view.DisplayStatusKey = "codespace.status." + view.DisplayStatus
	view.StatusSummaryKey = "codespace.status_summary." + view.DisplayStatus
	if view.DisplayStatus == DisplayStopped && !managerReady {
		view.StatusSummaryKey = "codespace.stopped_manager_unavailable"
	}
}

func applyRuntimeAccessView(codespace *codespace_model.Codespace, view *CreatorCodespaceView, manager *codespace_model.Manager, includeDetailData bool) bool {
	entry, hasEntry, err := getRuntimeMetadataEntry(codespace.ID)
	if err != nil || !hasEntry || !runtimeMetadataReadyForAccess(codespace, entry.Metadata) {
		return false
	}
	for _, endpoint := range entry.Metadata.Endpoints {
		if endpoint.EndpointID == workspaceEndpointID {
			view.Workspace = &CreatorEndpointView{
				EndpointID: workspaceEndpointID,
				OpenPath:   codespaceDetailPath(codespace.ID) + "/open",
			}
			continue
		}
		if !includeDetailData {
			continue
		}
		view.Endpoints = append(view.Endpoints, CreatorEndpointView{
			EndpointID: endpoint.EndpointID,
			Label:      endpoint.Label,
			Port:       uint16(endpoint.Port),
			Public:     endpoint.Public,
			URL:        endpoint.URL,
			OpenPath:   codespaceDetailPath(codespace.ID) + "/open/" + endpoint.EndpointID,
		})
	}
	if includeDetailData {
		view.ResourceUsage = creatorResourceUsageView(entry.Metadata.ResourceUsage)
		view.SSH = creatorSSHView(manager, entry.Metadata)
	}
	return true
}

func creatorSSHView(manager *codespace_model.Manager, metadata runtimeMetadata) *CreatorSSHView {
	ssh, err := parseRuntimeSSHURI(metadata.SSH)
	if err != nil {
		return nil
	}
	remoteHost := ssh.Hostname()
	if strings.Contains(remoteHost, ":") {
		remoteHost = "[" + remoteHost + "]"
	}
	user := ssh.User.Username()
	command := "ssh " + user + "@" + remoteHost
	if ssh.Port() != "22" {
		command = fmt.Sprintf("ssh -p %s %s@%s", ssh.Port(), user, remoteHost)
		remoteHost = ssh.Host
	}
	remote := user + "@" + remoteHost
	return &CreatorSSHView{
		Command:            command,
		VSCodeURL:          template.URL((&url.URL{Scheme: "vscode", Host: "vscode-remote", Path: "/ssh-remote+" + remote + metadata.WorkspaceFolder}).String()),
		ZedURL:             template.URL((&url.URL{Scheme: "zed", Host: "ssh", Path: "/" + remote + metadata.WorkspaceFolder}).String()),
		HostKeyAlgorithm:   strings.TrimSpace(manager.GatewaySSHHostKeyAlgorithm),
		HostKeyFingerprint: strings.TrimSpace(manager.GatewaySSHHostKeyFingerprintSHA256),
	}
}

func creatorResourceUsageView(usage runtimeMetadataResourceUsage) *CreatorResourceUsageView {
	if usage.ObservedUnix <= 0 || time.Now().Unix()-usage.ObservedUnix >= int64(runtimeMetadataMaxAge/time.Second) {
		return nil
	}
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
	startup := hasActiveStartup(codespace, time.Now().Unix())
	view.CanOpen = view.Workspace != nil && (startup || (view.DisplayStatus == DisplayRunning && (!hasActiveOperation(codespace) || isQueuedIdleStop(codespace))))
	for i := range view.Endpoints {
		view.Endpoints[i].CanOpen = startup || (view.DisplayStatus == DisplayRunning && (!hasActiveOperation(codespace) || (!view.Endpoints[i].Public && isQueuedIdleStop(codespace))))
	}
	view.CanContinue = codespace.Status == codespace_model.StatusRunning && isQueuedIdleStop(codespace)
	view.CanStop = codespace.Status == codespace_model.StatusRunning && (!hasActiveOperation(codespace) || isQueuedIdleStop(codespace))
	view.CanResume = codespace.Status == codespace_model.StatusStopped && !hasActiveOperation(codespace) && view.DisplayStatus == DisplayStopped && manager != nil && manager.RuntimeState == codespace_model.ManagerRuntimeStateOnline && !isManagerOffline(manager)
	view.CanCancelResume = codespace.Status == codespace_model.StatusStopped && codespace_model.ActiveOperationType(codespace) == codespace_model.OperationResume && codespace_model.IsOperationQueued(codespace)
}

func codespaceDetailPath(codespaceID int64) string {
	return "/-/codespaces/" + strconv.FormatInt(codespaceID, 10)
}
