// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
)

const (
	fetchMaxOperations         = 256
	fetchMaxObservedOperations = 10000
	fetchMaxQueuedCandidates   = 1024
)

const (
	// AcceptedOperationCreate means this Fetch request accepts create operations.
	AcceptedOperationCreate = "create"
	// AcceptedOperationResume means this Fetch request accepts resume operations.
	AcceptedOperationResume = "resume"
)

const (
	// OperationCommandAbortCreate means a running create should be reduced to local cleanup.
	OperationCommandAbortCreate = "abort_create"
	// OperationCommandAbortResume means a running resume should be reduced to local stop cleanup.
	OperationCommandAbortResume = "abort_resume"
)

var (
	// ErrFetchStateHistoryConflict is returned when observed operation history is ahead of Gitea.
	ErrFetchStateHistoryConflict = errors.New("codespace operation history conflict")
	// ErrFetchManagerUnavailable is returned when the Manager is not currently online.
	ErrFetchManagerUnavailable = errors.New("codespace manager unavailable")
)

// FetchOperationsOptions contains one Manager operation fetch request.
type FetchOperationsOptions struct {
	CapacityAvailable        int32
	AcceptedOperationTypes   []string
	MaxOperations            int32
	ObservedOperations       []ObservedOperation
	CleanupCapacityAvailable int32
}

// ObservedOperation identifies one operation already being handled by the Manager.
type ObservedOperation struct {
	CodespaceUUID     string
	OperationRVersion int64
}

// FetchOperationsResult contains payloads and renewed leases for a Manager.
type FetchOperationsResult struct {
	Operations    []OperationPayload
	RenewedLeases []RenewedOperationLease
}

// RenewedOperationLease contains the current lease for an observed operation.
type RenewedOperationLease struct {
	CodespaceUUID             string
	OperationRVersion         int64
	LeaseValidForMilliseconds int64
}

// OperationPayload contains one Manager operation command.
type OperationPayload struct {
	OperationRVersion         int64
	CodespaceUUID             string
	LogOffset                 int64
	LeaseValidForMilliseconds int64
	Command                   string
	Create                    *CreateOperationPayload
	Resume                    *ResumeOperationPayload
}

// CreateOperationPayload contains create-only repository data.
type CreateOperationPayload struct {
	RepoID             int64
	RepoFullName       string
	RepoName           string
	RepoCloneHTTPURL   string
	RepoCloneSSHURL    string
	RepoWebURL         string
	OwnerID            int64
	OwnerName          string
	OwnerType          string
	OwnerDisplayName   string
	CodespaceOwnerName string
	StartRef           string
	RefType            string
	RefName            string
	CommitSHA          string
	RepoTag            string
	RuntimeSettings    RuntimeSettings
	GitProtocol        string
}

// ResumeOperationPayload contains resume-only runtime data.
type ResumeOperationPayload struct {
	RuntimeSettings RuntimeSettings
	GitProtocol     string
}

// RuntimeSettings contains the effective runtime policy sent to Manager.
type RuntimeSettings struct {
	AutoStopEnabled       bool
	IdleTimeoutSeconds    int64
	InteractionGeneration int64
}

// FetchOperations renews observed operations and claims queued operations for one Manager.
func FetchOperations(ctx context.Context, manager *codespace_model.Manager, opts FetchOperationsOptions) (*FetchOperationsResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := validateFetchOptions(opts); err != nil {
		return nil, err
	}

	var result *FetchOperationsResult
	var summaries []*internalStateSummary
	err := globallock.LockAndDo(ctx, fetchManagerLockKey(manager.ID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentManager, err := loadFetchManager(ctx, manager.ID)
			if err != nil {
				return err
			}
			if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
				return ErrFetchManagerUnavailable
			}
			managerTags, err := decodeManagerTags(currentManager)
			if err != nil {
				return err
			}
			result = &FetchOperationsResult{}
			observedVersions, err := validateObservedOperationHistory(ctx, currentManager.ID, opts.ObservedOperations)
			if err != nil {
				return err
			}
			if err := appendRunningOperations(ctx, currentManager.ID, observedVersions, opts.MaxOperations, result, &summaries); err != nil {
				return err
			}
			if int32(len(result.Operations)) >= opts.MaxOperations {
				return nil
			}
			grantTime := time.Now()
			remaining := int(opts.MaxOperations) - len(result.Operations)
			if opts.CleanupCapacityAvailable > 0 {
				claimed, err := claimQueuedOperations(ctx, currentManager.ID, currentManager.OwnerID, grantTime, remaining, int(opts.CleanupCapacityAvailable), nil, []string{codespace_model.OperationStop, codespace_model.OperationDelete}, result, &summaries)
				if err != nil {
					return err
				}
				remaining -= claimed
			}
			if remaining <= 0 || opts.CapacityAvailable <= 0 || !setting.Codespace.Enabled {
				return nil
			}
			startupTypes := make([]string, 0, 2)
			if slices.Contains(opts.AcceptedOperationTypes, AcceptedOperationCreate) {
				startupTypes = append(startupTypes, codespace_model.OperationCreate)
			}
			if slices.Contains(opts.AcceptedOperationTypes, AcceptedOperationResume) {
				startupTypes = append(startupTypes, codespace_model.OperationResume)
			}
			if len(startupTypes) == 0 {
				return nil
			}
			_, err = claimQueuedOperations(ctx, currentManager.ID, currentManager.OwnerID, grantTime, remaining, int(opts.CapacityAvailable), managerTags, startupTypes, result, &summaries)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	appendInternalStateSummaries(ctx, summaries)
	return result, nil
}

func validateFetchOptions(opts FetchOperationsOptions) error {
	if opts.CapacityAvailable < 0 || opts.CapacityAvailable > 10000 {
		return errors.New("capacity_available must be between 0 and 10000")
	}
	if opts.CleanupCapacityAvailable < 0 || opts.CleanupCapacityAvailable > 256 {
		return errors.New("cleanup_capacity_available must be between 0 and 256")
	}
	if opts.MaxOperations < 1 || opts.MaxOperations > fetchMaxOperations {
		return errors.New("max_operations must be between 1 and 256")
	}
	if len(opts.ObservedOperations) > fetchMaxObservedOperations {
		return errors.New("observed_operations exceeds 10000")
	}
	seen := make(map[string]struct{}, len(opts.ObservedOperations))
	for _, observed := range opts.ObservedOperations {
		if err := codespace_model.ValidateUUID(observed.CodespaceUUID); err != nil {
			return err
		}
		if observed.OperationRVersion <= 0 {
			return errors.New("observed operation_rversion must be positive")
		}
		if _, ok := seen[observed.CodespaceUUID]; ok {
			return errors.New("observed_operations contains duplicate codespace uuid")
		}
		seen[observed.CodespaceUUID] = struct{}{}
	}
	for _, acceptedType := range opts.AcceptedOperationTypes {
		if acceptedType != AcceptedOperationCreate && acceptedType != AcceptedOperationResume {
			return fmt.Errorf("invalid accepted operation type %q", acceptedType)
		}
	}
	return nil
}

func loadFetchManager(ctx context.Context, managerID int64) (*codespace_model.Manager, error) {
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrFetchManagerUnavailable
	}
	return manager, nil
}

func currentManagerAllowsOnlineOrRecovering(ctx context.Context, managerID int64) (bool, error) {
	currentManager, err := loadFetchManager(ctx, managerID)
	if err != nil {
		if errors.Is(err, ErrFetchManagerUnavailable) {
			return false, nil
		}
		return false, err
	}
	return managerAllowsOnlineOrRecovering(currentManager), nil
}

func managerAllowsOnlineOrRecovering(manager *codespace_model.Manager) bool {
	switch manager.RuntimeState {
	case codespace_model.ManagerRuntimeStateOnline, codespace_model.ManagerRuntimeStateRecovering:
		return !isManagerOffline(manager)
	default:
		return false
	}
}

func decodeManagerTags(manager *codespace_model.Manager) ([]string, error) {
	var tags []string
	if err := json.Unmarshal([]byte(manager.TagsJSON), &tags); err != nil {
		return nil, fmt.Errorf("decode manager tags: %w", err)
	}
	if len(tags) == 0 {
		return []string{"default"}, nil
	}
	return tags, nil
}

func isManagerOffline(manager *codespace_model.Manager) bool {
	return manager.LastOnlineUnix <= 0 || time.Now().Unix()-manager.LastOnlineUnix > int64(setting.Codespace.ManagerOfflineTimeout/time.Second)
}

func validateObservedOperationHistory(ctx context.Context, managerID int64, observed []ObservedOperation) (map[string]int64, error) {
	observedVersions := make(map[string]int64, len(observed))
	for _, item := range observed {
		observedVersions[item.CodespaceUUID] = item.OperationRVersion
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(item.CodespaceUUID).Get(codespace)
		if err != nil {
			return nil, err
		}
		if !has || codespace.ManagerID != managerID {
			continue
		}
		if item.OperationRVersion > codespace.OperationRVersion {
			return nil, ErrFetchStateHistoryConflict
		}
	}
	return observedVersions, nil
}

func appendRunningOperations(ctx context.Context, managerID int64, observedVersions map[string]int64, maxOperations int32, result *FetchOperationsResult, summaries *[]*internalStateSummary) error {
	var rows []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("manager_id = ? AND operation_status = ?", managerID, codespace_model.OperationStatusRunning).
		Asc("operation_created_unix", "uuid").
		Find(&rows); err != nil {
		return err
	}
	grantTime := time.Now()
	for _, codespace := range rows {
		leaseMillis, deadlineUnix, ok := grantLease(codespace.OperationStartedUnix, grantTime)
		if !ok {
			summary := operationTimeoutSummary(codespace, timeoutStatus(codespace.OperationType))
			if err := applyRunningTimeout(ctx, codespace, grantTime.Unix()); err != nil {
				return err
			}
			*summaries = append(*summaries, summary)
			continue
		}
		observedVersion, hasObserved := observedVersions[codespace.UUID]
		if !hasObserved {
			continue
		}
		if observedVersion > codespace.OperationRVersion {
			return ErrFetchStateHistoryConflict
		}
		if !setting.Codespace.Enabled && isStartupOperation(codespace.OperationType) {
			if int32(len(result.Operations)) >= maxOperations {
				continue
			}
			result.Operations = append(result.Operations, *buildAbortOperationPayload(codespace))
			continue
		}
		if _, err := db.GetEngine(ctx).ID(codespace.UUID).Cols("operation_deadline_unix").Update(&codespace_model.Codespace{OperationDeadlineUnix: deadlineUnix}); err != nil {
			return err
		}
		if observedVersion == codespace.OperationRVersion {
			result.RenewedLeases = append(result.RenewedLeases, RenewedOperationLease{
				CodespaceUUID:             codespace.UUID,
				OperationRVersion:         codespace.OperationRVersion,
				LeaseValidForMilliseconds: leaseMillis,
			})
			continue
		}
		if int32(len(result.Operations)) >= maxOperations {
			continue
		}
		payload, err := buildOperationPayload(ctx, codespace, leaseMillis)
		if err != nil {
			return err
		}
		result.Operations = append(result.Operations, *payload)
	}
	return nil
}

func isStartupOperation(operationType string) bool {
	return operationType == codespace_model.OperationCreate || operationType == codespace_model.OperationResume
}

func claimQueuedOperations(ctx context.Context, managerID, managerOwnerID int64, grantTime time.Time, remaining, capacity int, managerTags, operationTypes []string, result *FetchOperationsResult, summaries *[]*internalStateSummary) (int, error) {
	if remaining <= 0 || capacity <= 0 {
		return 0, nil
	}
	limit := min(fetchMaxQueuedCandidates, remaining*4, capacity*4)
	var candidates []*codespace_model.Codespace
	query := db.GetEngine(ctx).
		Where("operation_status = ?", codespace_model.OperationStatusQueued).
		In("operation_type", operationTypes).
		In("status", queuedOperationCandidateStatuses(operationTypes)).
		Asc("operation_created_unix", "uuid").
		Limit(limit)
	if managerTags == nil {
		query = query.And("manager_id = ?", managerID)
	} else {
		query = query.In("manager_id", 0, managerID).In("repo_tag", managerTags)
	}
	if err := query.Find(&candidates); err != nil {
		return 0, err
	}
	claimed := 0
	for _, candidate := range candidates {
		if claimed >= capacity || claimed >= remaining {
			break
		}
		if isQueuedExpired(candidate, grantTime) {
			summary := operationTimeoutSummary(candidate, queuedTimeoutStatus(candidate.OperationType))
			if err := applyQueuedTimeout(ctx, candidate, grantTime.Unix()); err != nil {
				return claimed, err
			}
			*summaries = append(*summaries, summary)
			continue
		}
		if managerTags != nil && candidate.OperationType == codespace_model.OperationCreate && candidate.ManagerID != 0 {
			continue
		}
		if candidate.OperationType == codespace_model.OperationCreate {
			matches, err := createScopeMatches(ctx, managerOwnerID, candidate)
			if err != nil {
				return claimed, err
			}
			if !matches {
				continue
			}
		}
		if managerTags != nil && candidate.OperationType == codespace_model.OperationResume && candidate.ManagerID != managerID {
			continue
		}
		startedUnix := grantTime.Unix()
		leaseMillis, deadlineUnix, ok := grantLease(startedUnix, grantTime)
		if !ok {
			continue
		}
		affected, err := claimQueuedOperation(ctx, candidate, managerID, startedUnix, deadlineUnix)
		if err != nil {
			return claimed, err
		}
		if affected == 0 {
			continue
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(candidate.UUID).Get(codespace)
		if err != nil {
			return claimed, err
		}
		if !has || !isCurrentRunningOperation(codespace, managerID, candidate.OperationRVersion) {
			continue
		}
		payload, err := buildOperationPayload(ctx, codespace, leaseMillis)
		if err != nil {
			return claimed, err
		}
		result.Operations = append(result.Operations, *payload)
		claimed++
	}
	return claimed, nil
}

func claimQueuedOperation(ctx context.Context, candidate *codespace_model.Codespace, managerID, startedUnix, deadlineUnix int64) (int64, error) {
	updates := &codespace_model.Codespace{
		ManagerID:             managerID,
		OperationStatus:       codespace_model.OperationStatusRunning,
		OperationStartedUnix:  startedUnix,
		OperationDeadlineUnix: deadlineUnix,
	}
	query := db.GetEngine(ctx).
		Where("uuid = ? AND operation_r_version = ? AND operation_type = ? AND operation_status = ? AND operation_trigger = ?",
			candidate.UUID, candidate.OperationRVersion, candidate.OperationType, codespace_model.OperationStatusQueued, candidate.OperationTrigger)
	if candidate.OperationType == codespace_model.OperationCreate {
		query = query.And("status = ? AND manager_id = ?", codespace_model.StatusCreating, 0)
	} else {
		query = query.And("manager_id = ?", managerID)
	}
	return query.Cols("manager_id", "operation_status", "operation_started_unix", "operation_deadline_unix").Update(updates)
}

func queuedOperationCandidateStatuses(operationTypes []string) []string {
	statuses := make([]string, 0, len(operationTypes))
	for _, operationType := range operationTypes {
		switch operationType {
		case codespace_model.OperationCreate:
			statuses = append(statuses, codespace_model.StatusCreating)
		case codespace_model.OperationResume:
			statuses = append(statuses, codespace_model.StatusStopped)
		case codespace_model.OperationStop:
			statuses = append(statuses, codespace_model.StatusRunning)
		case codespace_model.OperationDelete:
			statuses = append(statuses, codespace_model.StatusDeleting)
		}
	}
	return statuses
}

func createScopeMatches(ctx context.Context, managerOwnerID int64, codespace *codespace_model.Codespace) (bool, error) {
	if managerOwnerID == 0 {
		return true, nil
	}
	repository, err := repo_model.GetRepositoryByID(ctx, codespace.RepoID)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return repository.OwnerID == managerOwnerID, nil
}

func isQueuedExpired(codespace *codespace_model.Codespace, now time.Time) bool {
	return codespace.OperationCreatedUnix > 0 && now.Unix() >= codespace.OperationCreatedUnix+int64(setting.Codespace.QueueTimeout/time.Second)
}

func applyQueuedTimeout(ctx context.Context, codespace *codespace_model.Codespace, now int64) error {
	return applyFinalState(ctx, codespace, queuedTimeoutStatus(codespace.OperationType), now)
}

func queuedTimeoutStatus(operationType string) string {
	switch operationType {
	case codespace_model.OperationResume:
		return codespace_model.StatusStopped
	case codespace_model.OperationStop:
		return codespace_model.StatusRunning
	default:
		return codespace_model.StatusFailed
	}
}

func applyRunningTimeout(ctx context.Context, codespace *codespace_model.Codespace, now int64) error {
	return applyFinalState(ctx, codespace, timeoutStatus(codespace.OperationType), now)
}

func grantLease(operationStartedUnix int64, grantTime time.Time) (int64, int64, bool) {
	if operationStartedUnix <= 0 {
		operationStartedUnix = grantTime.Unix()
	}
	totalDeadline := time.Unix(operationStartedUnix, 0).Add(setting.Codespace.OperationMaxDuration)
	remaining := totalDeadline.Sub(grantTime)
	if remaining < time.Millisecond {
		return 0, 0, false
	}
	lease := min(remaining, setting.Codespace.OperationLeaseTimeout)
	leaseMillis := lease.Milliseconds()
	leaseDeadlineUnix := ceilUnix(grantTime.Add(lease))
	totalDeadlineUnix := totalDeadline.Unix()
	deadlineUnix := min(totalDeadlineUnix, leaseDeadlineUnix)
	return leaseMillis, deadlineUnix, leaseMillis > 0
}

func ceilUnix(t time.Time) int64 {
	unix := t.Unix()
	if t.After(time.Unix(unix, 0)) {
		return unix + 1
	}
	return unix
}

func buildOperationPayload(ctx context.Context, codespace *codespace_model.Codespace, leaseMillis int64) (*OperationPayload, error) {
	payload := &OperationPayload{
		OperationRVersion:         codespace.OperationRVersion,
		CodespaceUUID:             codespace.UUID,
		LogOffset:                 codespace.LogSize,
		LeaseValidForMilliseconds: leaseMillis,
		Command:                   codespace.OperationType,
	}
	switch codespace.OperationType {
	case codespace_model.OperationCreate:
		create, err := buildCreatePayload(ctx, codespace)
		if err != nil {
			return nil, err
		}
		payload.Create = create
	case codespace_model.OperationResume:
		payload.Resume = &ResumeOperationPayload{
			RuntimeSettings: effectiveRuntimeSettings(codespace),
			GitProtocol:     codespace.GitProtocol,
		}
	case codespace_model.OperationStop, codespace_model.OperationDelete:
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported operation type %q", codespace.OperationType)
	}
	return payload, nil
}

func buildAbortOperationPayload(codespace *codespace_model.Codespace) *OperationPayload {
	command := OperationCommandAbortCreate
	if codespace.OperationType == codespace_model.OperationResume {
		command = OperationCommandAbortResume
	}
	return &OperationPayload{
		OperationRVersion: codespace.OperationRVersion,
		CodespaceUUID:     codespace.UUID,
		LogOffset:         codespace.LogSize,
		Command:           command,
	}
}

func buildCreatePayload(ctx context.Context, codespace *codespace_model.Codespace) (*CreateOperationPayload, error) {
	repository, err := repo_model.GetRepositoryByID(ctx, codespace.RepoID)
	if err != nil {
		return nil, err
	}
	owner, err := user_model.GetUserByID(ctx, repository.OwnerID)
	if err != nil {
		return nil, err
	}
	codespaceOwner, err := user_model.GetUserByID(ctx, codespace.UserID)
	if err != nil {
		return nil, err
	}
	cloneLink := repository.CloneLinkGeneral(ctx)
	capabilities, err := resolveGitTransportCapabilities(codespace.GitProtocol)
	if err != nil {
		return nil, err
	}
	httpCloneURL := cloneLink.HTTPS
	if !capabilities.HTTPEnabled {
		httpCloneURL = ""
	}
	sshCloneURL := cloneLink.SSH
	if !capabilities.SSHEnabled {
		sshCloneURL = ""
	}
	return &CreateOperationPayload{
		RepoID:             repository.ID,
		RepoFullName:       repository.FullName(),
		RepoName:           repository.Name,
		RepoCloneHTTPURL:   httpCloneURL,
		RepoCloneSSHURL:    sshCloneURL,
		RepoWebURL:         repository.HTMLURL(ctx),
		OwnerID:            owner.ID,
		OwnerName:          owner.Name,
		OwnerType:          ownerType(owner),
		OwnerDisplayName:   owner.DisplayName(),
		CodespaceOwnerName: codespaceOwner.Name,
		StartRef:           codespace.RefName,
		RefType:            codespace.RefType,
		RefName:            codespace.RefName,
		CommitSHA:          codespace.CommitSHA,
		RepoTag:            codespace.RepoTag,
		RuntimeSettings:    effectiveRuntimeSettings(codespace),
		GitProtocol:        codespace.GitProtocol,
	}, nil
}

func ownerType(owner *user_model.User) string {
	if owner.IsOrganization() {
		return "organization"
	}
	return "user"
}

func effectiveRuntimeSettings(codespace *codespace_model.Codespace) RuntimeSettings {
	settings := RuntimeSettings{
		AutoStopEnabled:       setting.Codespace.Enabled,
		IdleTimeoutSeconds:    int64(setting.Codespace.AutoStopDefaultTimeout / time.Second),
		InteractionGeneration: codespace.InteractionGeneration,
	}
	if !settings.AutoStopEnabled {
		settings.IdleTimeoutSeconds = 0
		return settings
	}
	switch codespace.AutoStopMode {
	case codespace_model.AutoStopModeNever:
		settings.AutoStopEnabled = false
		settings.IdleTimeoutSeconds = 0
	case codespace_model.AutoStopModeCustom:
		settings.IdleTimeoutSeconds = codespace.AutoStopTimeoutSeconds
	}
	if settings.AutoStopEnabled && settings.IdleTimeoutSeconds <= 0 {
		settings.IdleTimeoutSeconds = int64(setting.Codespace.AutoStopDefaultTimeout / time.Second)
	}
	return settings
}

func fetchManagerLockKey(managerID int64) string {
	return fmt.Sprintf("codespace_fetch_manager_%d", managerID)
}
