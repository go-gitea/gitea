// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

const (
	operationLeaseDuration     = 5 * time.Minute
	fetchMaxOperations         = 256
	fetchMaxObservedOperations = 10000
	fetchMaxQueuedCandidates   = 1024
)

var (
	// ErrFetchStateHistoryConflict is returned when observed operation history is ahead of Gitea.
	ErrFetchStateHistoryConflict = errors.New("codespace operation history conflict")
	// ErrFetchManagerUnavailable is returned when the Manager has no active coordination state or heartbeat.
	ErrFetchManagerUnavailable = errors.New("codespace manager unavailable")
)

// FetchOperationsOptions contains one Manager operation fetch request.
type FetchOperationsOptions struct {
	StartupCapacityAvailable int32
	AcceptedCreateTags       []string
	ObservedOperations       []*codespacev1.ObservedOperation
	CleanupCapacityAvailable int32
}

// RuntimeSettings contains the effective runtime policy sent to Manager.
type RuntimeSettings struct {
	AutoStopEnabled       bool
	IdleTimeoutSeconds    int64
	InteractionGeneration int64
}

// FetchOperations renews observed operations and claims queued operations for one Manager.
func FetchOperations(ctx context.Context, manager *codespace_model.Manager, opts FetchOperationsOptions) (*codespacev1.FetchOperationsResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := validateFetchOptions(opts); err != nil {
		return nil, util.NewInvalidArgumentErrorf("%v", err)
	}
	var result *codespacev1.FetchOperationsResponse
	var summaries []*internalStateSummary
	err := globallock.LockAndDo(ctx, fetchManagerLockKey(manager.ID), func(ctx context.Context) error {
		currentManager, err := loadCodespaceManager(ctx, manager.ID)
		if err != nil {
			return err
		}
		if !managerAllowsOnlineOrRecovering(currentManager) {
			return ErrFetchManagerUnavailable
		}
		managerEnvironments, err := decodeManagerEnvironments(currentManager)
		if err != nil {
			return err
		}
		acceptedCreateTags, err := normalizeAcceptedCreateTags(opts.AcceptedCreateTags, managerEnvironments)
		if err != nil {
			return util.NewInvalidArgumentErrorf("%v", err)
		}
		result = &codespacev1.FetchOperationsResponse{}
		running, observedVersions, err := loadRunningAndObservedOperations(ctx, currentManager.ID, opts.ObservedOperations)
		if err != nil {
			return err
		}
		maxOperations := max(int32(1), min(fetchMaxOperations, opts.StartupCapacityAvailable+opts.CleanupCapacityAvailable))
		if err := appendRunningOperations(ctx, running, observedVersions, maxOperations, result, &summaries); err != nil {
			return err
		}
		if currentManager.RuntimeState == codespace_model.ManagerRuntimeStateRecovering {
			return nil // Recovery may renew observed work, but cannot claim queued operations.
		}
		if int32(len(result.Operations)) >= maxOperations {
			return nil
		}
		grantTime := time.Now()
		remaining := int(maxOperations) - len(result.Operations)
		if opts.CleanupCapacityAvailable > 0 {
			claimed, err := claimQueuedOperations(ctx, currentManager.ID, currentManager.UserID, grantTime, remaining, int(opts.CleanupCapacityAvailable), nil, []string{codespace_model.OperationStop, codespace_model.OperationDelete}, result)
			if err != nil {
				return err
			}
			remaining -= claimed
		}
		if remaining <= 0 || opts.StartupCapacityAvailable <= 0 || !setting.Codespace.Enabled {
			return nil
		}
		capacity := int(opts.StartupCapacityAvailable)
		if len(acceptedCreateTags) > 0 {
			claimed, err := claimQueuedOperations(ctx, currentManager.ID, currentManager.UserID, grantTime, remaining, capacity, acceptedCreateTags, []string{codespace_model.OperationCreate}, result)
			if err != nil {
				return err
			}
			remaining -= claimed
			capacity -= claimed
		}
		if remaining > 0 && capacity > 0 {
			_, err = claimQueuedOperations(ctx, currentManager.ID, currentManager.UserID, grantTime, remaining, capacity, nil, []string{codespace_model.OperationResume}, result)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	for _, summary := range summaries {
		appendInternalStateSummary(ctx, summary)
	}
	return result, nil
}

func validateFetchOptions(opts FetchOperationsOptions) error {
	if opts.StartupCapacityAvailable < 0 || opts.StartupCapacityAvailable > 10000 {
		return errors.New("startup_capacity_available must be between 0 and 10000")
	}
	if opts.CleanupCapacityAvailable < 0 || opts.CleanupCapacityAvailable > 256 {
		return errors.New("cleanup_capacity_available must be between 0 and 256")
	}
	if len(opts.AcceptedCreateTags) > managerMaxEnvironments {
		return errors.New("accepted_create_tags exceeds 64")
	}
	if len(opts.ObservedOperations) > fetchMaxObservedOperations {
		return errors.New("observed_operations exceeds 10000")
	}
	seen := make(map[int64]struct{}, len(opts.ObservedOperations))
	for _, observed := range opts.ObservedOperations {
		if observed == nil {
			return errors.New("observed operation is required")
		}
		if observed.GetCodespaceId() <= 0 {
			return errors.New("codespace_id must be positive")
		}
		if observed.GetOperationRversion() <= 0 {
			return errors.New("observed operation_rversion must be positive")
		}
		if _, ok := seen[observed.GetCodespaceId()]; ok {
			return errors.New("observed_operations contains duplicate codespace id")
		}
		seen[observed.GetCodespaceId()] = struct{}{}
	}
	return nil
}

func loadCodespaceManager(ctx context.Context, managerID int64) (*codespace_model.Manager, error) {
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
	currentManager, err := loadCodespaceManager(ctx, managerID)
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

func decodeManagerEnvironments(manager *codespace_model.Manager) ([]ManagerEnvironmentDeclaration, error) {
	var environments []ManagerEnvironmentDeclaration
	if err := json.Unmarshal([]byte(manager.TagsJSON), &environments); err != nil {
		return nil, fmt.Errorf("decode manager environments: %w", err)
	}
	return environments, nil
}

func normalizeAcceptedCreateTags(tags []string, declared []ManagerEnvironmentDeclaration) ([]string, error) {
	declaredTags := make(map[string]struct{}, len(declared))
	for _, environment := range declared {
		declaredTags[environment.Tag] = struct{}{}
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if !environmentTagPattern.MatchString(tag) {
			return nil, fmt.Errorf("invalid accepted create tag %q", tag)
		}
		if _, ok := declaredTags[tag]; !ok {
			return nil, fmt.Errorf("accepted create tag %q is not declared", tag)
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

func isManagerOffline(manager *codespace_model.Manager) bool {
	return manager.LastOnlineUnix <= 0 || time.Now().Unix()-manager.LastOnlineUnix > int64(managerOfflineTimeout/time.Second)
}

func loadRunningAndObservedOperations(ctx context.Context, managerID int64, observed []*codespacev1.ObservedOperation) ([]*codespace_model.Codespace, map[int64]int64, error) {
	var running []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("manager_id = ? AND operation_created_unix > 0 AND operation_started_unix > 0", managerID).
		Asc("operation_created_unix", "id").Select(codespaceStateColumns).Find(&running); err != nil {
		return nil, nil, err
	}
	versions := make(map[int64]int64, len(running))
	for _, row := range running {
		versions[row.ID] = row.OperationRVersion
	}
	observedVersions := make(map[int64]int64, len(observed))
	var missing []int64
	for _, item := range observed {
		observedVersions[item.GetCodespaceId()] = item.GetOperationRversion()
		if _, ok := versions[item.GetCodespaceId()]; !ok {
			missing = append(missing, item.GetCodespaceId())
		}
	}
	for ids := range slices.Chunk(missing, db.DefaultMaxInSize) {
		var rows []*codespace_model.Codespace
		if err := db.GetEngine(ctx).Where("manager_id = ?", managerID).In("id", ids).Cols("id", "operation_r_version").Find(&rows); err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			versions[row.ID] = row.OperationRVersion
		}
	}
	for id, version := range observedVersions {
		if current, ok := versions[id]; ok && version > current {
			return nil, nil, ErrFetchStateHistoryConflict
		}
	}
	return running, observedVersions, nil
}

func appendRunningOperations(ctx context.Context, rows []*codespace_model.Codespace, observedVersions map[int64]int64, maxOperations int32, result *codespacev1.FetchOperationsResponse, summaries *[]*internalStateSummary) error {
	grantTime := time.Now()
	for _, codespace := range rows {
		if err := codespace_model.ValidateCodespaceLifecycle(codespace); err != nil {
			return fmt.Errorf("invalid persisted Codespace: %w", err)
		}
		remainingLease := time.Unix(codespace.OperationDeadlineUnix, 0).Sub(grantTime)
		if codespace.OperationDeadlineUnix <= 0 || remainingLease < time.Millisecond {
			summary, err := db.WithTx2(ctx, func(ctx context.Context) (*internalStateSummary, error) {
				return applyRunningTimeout(ctx, codespace, grantTime.Unix())
			})
			if err != nil {
				if errors.Is(err, errCodespaceStateChanged) {
					continue
				}
				return err
			}
			*summaries = append(*summaries, summary)
			continue
		}
		observedVersion, hasObserved := observedVersions[codespace.ID]
		operationType := codespace_model.ActiveOperationType(codespace)
		if hasObserved && !setting.Codespace.Enabled && (operationType == codespace_model.OperationCreate || operationType == codespace_model.OperationResume) {
			if int32(len(result.Operations)) < maxOperations {
				result.Operations = append(result.Operations, buildAbortOperationPayload(codespace))
			}
			continue
		}
		if hasObserved && observedVersion == codespace.OperationRVersion {
			leaseMillis := remainingLease.Milliseconds()
			if remainingLease <= operationLeaseDuration/2 {
				granted, deadlineUnix, ok := grantLease(codespace.OperationStartedUnix, grantTime)
				if !ok {
					summary, err := db.WithTx2(ctx, func(ctx context.Context) (*internalStateSummary, error) {
						return applyRunningTimeout(ctx, codespace, grantTime.Unix())
					})
					if err != nil {
						if errors.Is(err, errCodespaceStateChanged) {
							continue
						}
						return err
					}
					*summaries = append(*summaries, summary)
					continue
				}
				if deadlineUnix != codespace.OperationDeadlineUnix {
					expected := snapshotCodespaceState(codespace)
					codespace.OperationDeadlineUnix = deadlineUnix
					if err := updateCodespaceIfCurrent(ctx, expected, codespace, "operation_deadline_unix"); err != nil {
						if errors.Is(err, errCodespaceStateChanged) {
							continue
						}
						return err
					}
					leaseMillis = granted
				}
			}
			result.RenewedLeases = append(result.RenewedLeases, &codespacev1.RenewedOperationLease{
				CodespaceId:               codespace.ID,
				OperationRversion:         codespace.OperationRVersion,
				LeaseValidForMilliseconds: leaseMillis,
			})
			continue
		}
		if int32(len(result.Operations)) >= maxOperations {
			continue
		}
		payload, err := buildOperationPayload(ctx, codespace, remainingLease.Milliseconds())
		if err != nil {
			return err
		}
		result.Operations = append(result.Operations, payload)
	}
	return nil
}

func claimQueuedOperations(ctx context.Context, managerID, managerUserID int64, grantTime time.Time, remaining, capacity int, createTags, operationTypes []string, result *codespacev1.FetchOperationsResponse) (int, error) {
	if remaining <= 0 || capacity <= 0 {
		return 0, nil
	}
	limit := min(fetchMaxQueuedCandidates, remaining*4, capacity*4)
	var candidates []*codespace_model.Codespace
	query := db.GetEngine(ctx).
		Where("operation_created_unix > 0 AND operation_started_unix = 0 AND operation_deadline_unix = 0").
		In("status", queuedOperationCandidateStatuses(operationTypes)).
		Asc("operation_created_unix", "id").
		Limit(limit)
	if createTags == nil {
		query = query.And("manager_id = ?", managerID)
	} else {
		query = query.And("manager_id = ? AND repo_id > ?", 0, 0).In("environment_tag", createTags)
		if managerUserID > 0 {
			query = query.And("user_id = ?", managerUserID)
		}
	}
	if err := query.Select(codespaceStateColumns).Find(&candidates); err != nil {
		return 0, err
	}
	claimed := 0
	for _, candidate := range candidates {
		if claimed >= capacity || claimed >= remaining {
			break
		}
		startedUnix := grantTime.Unix()
		leaseMillis, deadlineUnix, ok := grantLease(startedUnix, grantTime)
		if !ok {
			continue
		}
		affected, err := claimQueuedOperation(ctx, candidate, managerID, managerUserID, startedUnix, deadlineUnix)
		if err != nil {
			return claimed, err
		}
		if affected == 0 {
			continue
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(candidate.ID).Select(codespaceStateColumns).Get(codespace)
		if err != nil {
			return claimed, err
		}
		if !has || !isCurrentRunningOperation(codespace, managerID, candidate.OperationRVersion) {
			continue
		}
		payload, err := buildOperationPayload(ctx, codespace, leaseMillis)
		if err != nil {
			operationType := codespace_model.ActiveOperationType(codespace)
			query := db.GetEngine(ctx).
				Where("id = ? AND manager_id = ? AND operation_r_version = ? AND status = ? AND operation_trigger = ? AND operation_started_unix = ? AND operation_deadline_unix = ?",
					codespace.ID, managerID, codespace.OperationRVersion, codespace.Status, codespace.OperationTrigger, codespace.OperationStartedUnix, codespace.OperationDeadlineUnix)
			columns := []string{"operation_started_unix", "operation_deadline_unix"}
			updates := &codespace_model.Codespace{}
			if operationType == codespace_model.OperationCreate {
				query = query.And("status = ?", codespace_model.StatusCreating)
				updates.ManagerID = 0
				columns = append(columns, "manager_id")
			}
			if _, releaseErr := query.Cols(columns...).Update(updates); releaseErr != nil {
				return claimed, fmt.Errorf("build operation payload: %w; release claim: %v", err, releaseErr)
			}
			return claimed, err
		}
		result.Operations = append(result.Operations, payload)
		claimed++
	}
	return claimed, nil
}

func claimQueuedOperation(ctx context.Context, candidate *codespace_model.Codespace, managerID, managerUserID, startedUnix, deadlineUnix int64) (int64, error) {
	updates := &codespace_model.Codespace{
		ManagerID:             managerID,
		OperationStartedUnix:  startedUnix,
		OperationDeadlineUnix: deadlineUnix,
	}
	// Keep every scheduling predicate in the UPDATE so concurrent Managers cannot both claim a stale candidate.
	query := db.GetEngine(ctx).
		Where("id = ? AND operation_r_version = ? AND status = ? AND operation_trigger = ? AND operation_created_unix = ? AND operation_started_unix = 0 AND operation_deadline_unix = 0",
			candidate.ID, candidate.OperationRVersion, candidate.Status, candidate.OperationTrigger, candidate.OperationCreatedUnix)
	if codespace_model.ActiveOperationType(candidate) == codespace_model.OperationCreate {
		query = query.And("status = ? AND manager_id = ? AND environment_tag = ? AND repo_id = ?", codespace_model.StatusCreating, 0, candidate.EnvironmentTag, candidate.RepoID)
		if managerUserID > 0 {
			query = query.And("user_id = ?", managerUserID)
		}
	} else {
		query = query.And("manager_id = ?", managerID)
	}
	return query.Cols("manager_id", "operation_started_unix", "operation_deadline_unix").Update(updates)
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

func grantLease(operationStartedUnix int64, grantTime time.Time) (int64, int64, bool) {
	if operationStartedUnix <= 0 {
		operationStartedUnix = grantTime.Unix()
	}
	totalDeadline := time.Unix(operationStartedUnix, 0).Add(setting.Codespace.OperationMaxDuration)
	remaining := totalDeadline.Sub(grantTime)
	if remaining < time.Millisecond {
		return 0, 0, false
	}
	lease := min(remaining, operationLeaseDuration)
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

func buildOperationPayload(ctx context.Context, codespace *codespace_model.Codespace, leaseMillis int64) (*codespacev1.OperationPayload, error) {
	if err := codespace_model.ValidateCodespaceLifecycle(codespace); err != nil {
		return nil, fmt.Errorf("invalid persisted Codespace: %w", err)
	}
	payload := &codespacev1.OperationPayload{
		OperationRversion:         codespace.OperationRVersion,
		CodespaceId:               codespace.ID,
		LeaseValidForMilliseconds: leaseMillis,
	}
	operationType := codespace_model.ActiveOperationType(codespace)
	switch operationType {
	case codespace_model.OperationCreate:
		var configuration codespace_model.Codespace
		has, err := db.GetEngine(ctx).ID(codespace.ID).Cols("dev_container_path", "dev_container_content").Get(&configuration)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, errCodespaceStateChanged
		}
		codespace.DevContainerPath = configuration.DevContainerPath
		codespace.DevContainerContent = configuration.DevContainerContent
		create, err := buildCreatePayload(ctx, codespace)
		if err != nil {
			return nil, err
		}
		payload.Command = &codespacev1.OperationPayload_Create{Create: create}
	case codespace_model.OperationResume:
		payload.Command = &codespacev1.OperationPayload_Resume{Resume: &codespacev1.ResumeOperationPayload{
			RuntimeSettings: runtimeSettingsMessage(effectiveRuntimeSettings(codespace)),
		}}
	case codespace_model.OperationStop:
		payload.Command = &codespacev1.OperationPayload_Stop{Stop: &codespacev1.StopOperationPayload{}}
	case codespace_model.OperationDelete:
		payload.Command = &codespacev1.OperationPayload_Delete{Delete: &codespacev1.DeleteOperationPayload{}}
	default:
		return nil, fmt.Errorf("unsupported operation type %q", operationType)
	}
	return payload, nil
}

func buildAbortOperationPayload(codespace *codespace_model.Codespace) *codespacev1.OperationPayload {
	payload := &codespacev1.OperationPayload{
		OperationRversion: codespace.OperationRVersion,
		CodespaceId:       codespace.ID,
	}
	if codespace_model.ActiveOperationType(codespace) == codespace_model.OperationResume {
		payload.Command = &codespacev1.OperationPayload_AbortResume{AbortResume: &codespacev1.AbortResumeOperationPayload{}}
	} else {
		payload.Command = &codespacev1.OperationPayload_AbortCreate{AbortCreate: &codespacev1.AbortCreateOperationPayload{}}
	}
	return payload
}

func buildCreatePayload(ctx context.Context, codespace *codespace_model.Codespace) (*codespacev1.CreateOperationPayload, error) {
	repository, err := repo_model.GetRepositoryByID(ctx, codespace.RepoID)
	if err != nil {
		return nil, err
	}
	codespaceOwner, err := user_model.GetUserByID(ctx, codespace.UserID)
	if err != nil {
		return nil, err
	}
	gitProtocol, err := createGitProtocol()
	if err != nil {
		return nil, err
	}
	cloneRepository := repository
	startRef, err := canonicalStartRef(codespace.RefType, codespace.RefName)
	if err != nil {
		return nil, err
	}
	if codespace.RefType == "pull" {
		index, err := pullIndexFromCanonicalRef(codespace.RefName)
		if err != nil {
			return nil, err
		}
		pr, err := issues_model.GetPullRequestByIndex(ctx, repository.ID, index)
		if err != nil {
			return nil, err
		}
		if pr.BaseRepoID != repository.ID {
			return nil, errors.New("persisted pull request base repository mismatch")
		}
		if !pr.IsAgitFlow() {
			if err := pr.LoadHeadRepo(ctx); err != nil {
				return nil, err
			}
			if pr.HeadRepo == nil {
				return nil, errors.New("pull request head repository not found")
			}
			if err := validateCreateRepository(pr.HeadRepo); err != nil {
				return nil, err
			}
			exists, err := git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, errors.New("pull request head branch not found")
			}
			canRead, err := access_model.HasAccessUnit(ctx, codespaceOwner, pr.HeadRepo, unit_model.TypeCode, perm_model.AccessModeRead)
			if err != nil {
				return nil, err
			}
			if !canRead {
				return nil, ErrCreatePermissionDenied
			}
			cloneRepository = pr.HeadRepo
			startRef = "refs/heads/" + pr.HeadBranch
		}
	}
	cloneLink := cloneRepository.CloneLinkGeneral(ctx)
	capabilities, err := resolveGitTransportCapabilities(gitProtocol)
	if err != nil {
		return nil, err
	}
	cloneURLs := make([]string, 0, 2)
	if capabilities.HTTPEnabled && cloneLink.HTTPS != "" {
		cloneURLs = append(cloneURLs, cloneLink.HTTPS)
	}
	if capabilities.SSHEnabled && cloneLink.SSH != "" {
		cloneURLs = append(cloneURLs, cloneLink.SSH)
	}
	if gitProtocol == codespace_model.GitProtocolSSH {
		slices.Reverse(cloneURLs)
	}
	if len(cloneURLs) == 0 {
		return nil, errors.New("codespace repository has no usable clone URL")
	}
	devContainer := &codespacev1.DevContainerConfiguration{}
	if strings.TrimSpace(codespace.DevContainerPath) != "" && strings.TrimSpace(codespace.DevContainerContent) == "" {
		devContainer.Source = &codespacev1.DevContainerConfiguration_RepositoryPath{RepositoryPath: codespace.DevContainerPath}
	} else if strings.TrimSpace(codespace.DevContainerPath) == "" && strings.TrimSpace(codespace.DevContainerContent) != "" {
		devContainer.Source = &codespacev1.DevContainerConfiguration_TemplateContent{TemplateContent: codespace.DevContainerContent}
	} else {
		return nil, errors.New("invalid persisted Dev Container configuration")
	}
	return &codespacev1.CreateOperationPayload{
		Repository: &codespacev1.RepositoryCheckout{
			RepositoryId: repository.ID,
			FullName:     repository.FullName(),
			CloneUrls:    cloneURLs,
			StartRef:     startRef,
			CommitSha:    codespace.CommitSHA,
		},
		EnvironmentTag: codespace.EnvironmentTag,
		GitIdentity: &codespacev1.GitIdentity{
			UserId:        codespaceOwner.ID,
			GiteaUsername: codespaceOwner.Name,
			GitUserEmail:  codespaceOwner.GetEmail(),
		},
		DevContainer:    devContainer,
		RuntimeSettings: runtimeSettingsMessage(effectiveRuntimeSettings(codespace)),
	}, nil
}

func pullIndexFromCanonicalRef(refName string) (int64, error) {
	value, ok := strings.CutPrefix(strings.TrimSpace(refName), "refs/pull/")
	if !ok {
		return 0, errors.New("invalid persisted pull request ref")
	}
	value, ok = strings.CutSuffix(value, "/head")
	if !ok {
		return 0, errors.New("invalid persisted pull request ref")
	}
	index, err := strconv.ParseInt(value, 10, 64)
	if err != nil || index <= 0 {
		return 0, errors.New("invalid persisted pull request ref")
	}
	return index, nil
}

func canonicalStartRef(refType, refName string) (string, error) {
	refName = strings.TrimSpace(refName)
	switch refType {
	case "branch":
		if refName != "" {
			return "refs/heads/" + refName, nil
		}
	case "tag":
		if refName != "" {
			return "refs/tags/" + refName, nil
		}
	case "pull":
		if strings.HasPrefix(refName, "refs/pull/") && strings.HasSuffix(refName, "/head") {
			return refName, nil
		}
	case "commit":
		return "", nil
	}
	return "", fmt.Errorf("invalid persisted codespace ref %q %q", refType, refName)
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

func runtimeSettingsMessage(settings RuntimeSettings) *codespacev1.EffectiveCodespaceRuntimeSettings {
	return &codespacev1.EffectiveCodespaceRuntimeSettings{
		IdleTimeoutSeconds:    settings.IdleTimeoutSeconds,
		InteractionGeneration: settings.InteractionGeneration,
	}
}

func fetchManagerLockKey(managerID int64) string {
	return fmt.Sprintf("codespace_fetch_manager_%d", managerID)
}
