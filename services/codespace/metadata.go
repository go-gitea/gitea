// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"

	"google.golang.org/protobuf/proto"
)

const (
	workspaceEndpointID       = "workspace"
	workspaceEndpointLabel    = "Workspace"
	bootStagePrepareRuntime   = "prepare-runtime"
	bootStageInitializeSystem = "initialize-system"
	bootStagePrepareWorkspace = "prepare-workspace"
	bootStageStartEnvironment = "start-environment"
	bootStagePublishReady     = "publish-ready"
	bootStageReady            = "ready"
)

var endpointIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$`)

var (
	// ErrRuntimeMetadataManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrRuntimeMetadataManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrRuntimeMetadataStaleOperation is returned when the snapshot no longer matches current lifecycle state.
	ErrRuntimeMetadataStaleOperation = errors.New("runtime metadata does not match current operation")
	// ErrRuntimeMetadataGenerationConflict is returned when one generation carries different content.
	ErrRuntimeMetadataGenerationConflict = errors.New("runtime metadata generation conflict")
	// ErrRuntimeMetadataVersionExhausted is returned when Gitea cannot provide a higher metadata generation.
	ErrRuntimeMetadataVersionExhausted = errors.New("runtime metadata generation exhausted")
	// ErrRuntimeMetadataManagerOffline is returned when the authenticated Manager is not usable for metadata writes.
	ErrRuntimeMetadataManagerOffline = errors.New("manager is not online")
	// ErrRuntimeMetadataStateUnavailable is returned when Codespace metadata writes are disabled.
	ErrRuntimeMetadataStateUnavailable = errors.New("codespace runtime metadata state unavailable")
)

// StaleGenerationError reports the server-side generation that supersedes a request.
type StaleGenerationError struct {
	CurrentGeneration int64
}

func (e *StaleGenerationError) Error() string {
	return fmt.Sprintf("runtime metadata generation is stale; current generation is %d", e.CurrentGeneration)
}

// ReportRuntimeMetadataOptions contains a Manager metadata report after RPC validation.
type ReportRuntimeMetadataOptions struct {
	CodespaceUUID      string
	Metadata           *codespacev1.RuntimeMetadata
	MetadataGeneration int64
}

type runtimeMetadataCacheEntry struct {
	Metadata         runtimeMetadata `json:"metadata"`
	Generation       int64           `json:"generation"`
	ContentHash      string          `json:"content_hash"`
	LastReportedUnix int64           `json:"last_reported_unix"`
}

type runtimeMetadata struct {
	Endpoints     []runtimeMetadataEndpoint    `json:"endpoints"`
	Boot          runtimeMetadataBoot          `json:"boot"`
	ResourceUsage runtimeMetadataResourceUsage `json:"resource_usage"`
}

type runtimeMetadataEndpoint struct {
	EndpointID string `json:"endpoint_id"`
	Label      string `json:"label"`
	Public     bool   `json:"public"`
}

func (m runtimeMetadata) endpointByID(endpointID string) (runtimeMetadataEndpoint, bool) {
	for _, endpoint := range m.Endpoints {
		if endpoint.EndpointID == endpointID {
			return endpoint, true
		}
	}
	return runtimeMetadataEndpoint{}, false
}

func runtimeMetadataReadyForRunning(codespace *codespace_model.Codespace, metadata runtimeMetadata) bool {
	return metadata.Boot.Stage == bootStageReady &&
		metadata.Boot.OperationRVersion <= codespace.OperationRVersion
}

type runtimeMetadataBoot struct {
	OperationRVersion int64  `json:"operation_rversion"`
	Stage             string `json:"stage"`
	StartedUnix       int64  `json:"started_unix"`
	LastUpdateUnix    int64  `json:"last_update_unix"`
}

type runtimeMetadataResourceUsage struct {
	CPU          runtimeMetadataCPUUsage    `json:"cpu"`
	Memory       runtimeMetadataMemoryUsage `json:"memory"`
	Disk         runtimeMetadataDiskUsage   `json:"disk"`
	ObservedUnix int64                      `json:"observed_unix"`
}

type runtimeMetadataCPUUsage struct {
	UsedMillicores  int64 `json:"used_millicores"`
	LimitMillicores int64 `json:"limit_millicores"`
}

type runtimeMetadataMemoryUsage struct {
	UsedBytes  int64 `json:"used_bytes"`
	LimitBytes int64 `json:"limit_bytes"`
}

type runtimeMetadataDiskUsage struct {
	UsedBytes  int64 `json:"used_bytes"`
	LimitBytes int64 `json:"limit_bytes"`
}

// ReportRuntimeMetadata validates and stores a Runtime Metadata snapshot in Gitea cache.
func ReportRuntimeMetadata(ctx context.Context, manager *codespace_model.Manager, opts ReportRuntimeMetadataOptions) error {
	if !setting.Codespace.Enabled {
		return ErrRuntimeMetadataStateUnavailable
	}
	if manager == nil || manager.ID <= 0 {
		return errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return err
	}
	if opts.MetadataGeneration <= 0 {
		return errors.New("metadata_generation must be positive")
	}
	allowed, err := currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRuntimeMetadataManagerOffline
	}
	metadata, contentHash, err := normalizeRuntimeMetadata(opts.Metadata)
	if err != nil {
		return err
	}

	return globallock.LockAndDo(ctx, runtimeMetadataLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		allowed, err = currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRuntimeMetadataManagerOffline
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			return ErrRuntimeMetadataStaleOperation
		}
		if codespace.ManagerID != manager.ID {
			return ErrRuntimeMetadataManagerMismatch
		}
		if err := validateRuntimeMetadataState(codespace, metadata); err != nil {
			return err
		}

		current, hasCurrent, err := getRuntimeMetadataEntry(opts.CodespaceUUID)
		if err != nil {
			return err
		}
		if hasCurrent {
			if opts.MetadataGeneration < current.Generation {
				if current.Generation == math.MaxInt64 {
					return ErrRuntimeMetadataVersionExhausted
				}
				return &StaleGenerationError{CurrentGeneration: current.Generation}
			}
			if opts.MetadataGeneration == current.Generation && contentHash != current.ContentHash {
				return ErrRuntimeMetadataGenerationConflict
			}
			if err := validateRuntimeMetadataStageForward(current.Metadata, metadata); err != nil {
				return err
			}
		}

		return putRuntimeMetadataEntry(opts.CodespaceUUID, runtimeMetadataCacheEntry{
			Metadata:         metadata,
			Generation:       opts.MetadataGeneration,
			ContentHash:      contentHash,
			LastReportedUnix: time.Now().Unix(),
		})
	})
}

// HasReadyRuntimeMetadata reports whether Gitea cache contains current operation ready metadata.
func HasReadyRuntimeMetadata(_ context.Context, codespaceUUID string, operationRVersion int64) (bool, error) {
	entry, has, err := getRuntimeMetadataEntry(codespaceUUID)
	if err != nil || !has {
		return false, err
	}
	return entry.Metadata.Boot.Stage == bootStageReady &&
		entry.Metadata.Boot.OperationRVersion == operationRVersion, nil
}

func deleteRuntimeMetadata(codespaceUUID string) {
	if cache.GetCache() == nil {
		return
	}
	_ = cache.GetCache().Delete(runtimeMetadataCacheKey(codespaceUUID))
}

func normalizeRuntimeMetadata(input *codespacev1.RuntimeMetadata) (runtimeMetadata, string, error) {
	if input == nil {
		return runtimeMetadata{}, "", errors.New("runtime metadata is required")
	}
	if int64(proto.Size(input)) > setting.Codespace.RuntimeMetadataMaxSize {
		return runtimeMetadata{}, "", errors.New("runtime metadata exceeds maximum size")
	}
	if input.GetBoot() == nil {
		return runtimeMetadata{}, "", errors.New("runtime metadata boot is required")
	}
	resourceUsage, err := normalizeRuntimeMetadataResourceUsage(input.GetResourceUsage())
	if err != nil {
		return runtimeMetadata{}, "", err
	}
	bootStage, ok := runtimeMetadataBootStage(input.GetBoot().GetStage())
	if !ok {
		return runtimeMetadata{}, "", fmt.Errorf("invalid boot stage %q", input.GetBoot().GetStage())
	}

	metadata := runtimeMetadata{
		Endpoints: make([]runtimeMetadataEndpoint, 0, len(input.GetEndpoints())),
		Boot: runtimeMetadataBoot{
			OperationRVersion: input.GetBoot().GetOperationRversion(),
			Stage:             bootStage,
			StartedUnix:       input.GetBoot().GetStartedUnix(),
			LastUpdateUnix:    input.GetBoot().GetLastUpdateUnix(),
		},
		ResourceUsage: resourceUsage,
	}
	if err := validateRuntimeMetadataBoot(metadata.Boot); err != nil {
		return runtimeMetadata{}, "", err
	}
	if len(input.GetEndpoints()) > 64 {
		return runtimeMetadata{}, "", errors.New("runtime metadata endpoints exceed 64")
	}
	seen := make(map[string]struct{}, len(input.GetEndpoints()))
	workspaceFound := false
	for _, endpoint := range input.GetEndpoints() {
		normalized, err := normalizeRuntimeMetadataEndpoint(endpoint)
		if err != nil {
			return runtimeMetadata{}, "", err
		}
		if _, ok := seen[normalized.EndpointID]; ok {
			return runtimeMetadata{}, "", fmt.Errorf("duplicate endpoint_id %q", normalized.EndpointID)
		}
		seen[normalized.EndpointID] = struct{}{}
		if normalized.EndpointID == workspaceEndpointID {
			workspaceFound = true
		}
		metadata.Endpoints = append(metadata.Endpoints, normalized)
	}
	if (bootStage == bootStagePublishReady || bootStage == bootStageReady) && !workspaceFound {
		return runtimeMetadata{}, "", errors.New("runtime metadata workspace endpoint is required")
	}
	slices.SortFunc(metadata.Endpoints, func(a, b runtimeMetadataEndpoint) int {
		return strings.Compare(a.EndpointID, b.EndpointID)
	})

	canonical, err := json.Marshal(metadata)
	if err != nil {
		return runtimeMetadata{}, "", fmt.Errorf("encode canonical runtime metadata: %w", err)
	}
	if int64(len(canonical)) > setting.Codespace.RuntimeMetadataMaxSize {
		return runtimeMetadata{}, "", errors.New("runtime metadata exceeds maximum size")
	}
	sum := sha256.Sum256(canonical)
	return metadata, hex.EncodeToString(sum[:]), nil
}

func normalizeRuntimeMetadataEndpoint(endpoint *codespacev1.RuntimeEndpoint) (runtimeMetadataEndpoint, error) {
	if endpoint == nil {
		return runtimeMetadataEndpoint{}, errors.New("runtime metadata endpoint is required")
	}
	if !endpointIDPattern.MatchString(endpoint.GetEndpointId()) {
		return runtimeMetadataEndpoint{}, fmt.Errorf("invalid endpoint_id %q", endpoint.GetEndpointId())
	}
	label, err := normalizeRuntimeMetadataLabel(endpoint.GetLabel())
	if err != nil {
		return runtimeMetadataEndpoint{}, err
	}
	normalized := runtimeMetadataEndpoint{
		EndpointID: endpoint.GetEndpointId(),
		Label:      label,
		Public:     endpoint.GetPublic(),
	}
	if normalized.EndpointID == workspaceEndpointID && (normalized.Label != workspaceEndpointLabel || normalized.Public) {
		return runtimeMetadataEndpoint{}, errors.New("runtime metadata workspace endpoint is invalid")
	}
	return normalized, nil
}

func normalizeRuntimeMetadataResourceUsage(input *codespacev1.RuntimeResourceUsage) (runtimeMetadataResourceUsage, error) {
	if input == nil {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata resource_usage is required")
	}
	if input.GetCpu() == nil {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata cpu usage is required")
	}
	if input.GetMemory() == nil {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata memory usage is required")
	}
	if input.GetDisk() == nil {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata disk usage is required")
	}
	usage := runtimeMetadataResourceUsage{
		CPU: runtimeMetadataCPUUsage{
			UsedMillicores:  input.GetCpu().GetUsedMillicores(),
			LimitMillicores: input.GetCpu().GetLimitMillicores(),
		},
		Memory: runtimeMetadataMemoryUsage{
			UsedBytes:  input.GetMemory().GetUsedBytes(),
			LimitBytes: input.GetMemory().GetLimitBytes(),
		},
		Disk: runtimeMetadataDiskUsage{
			UsedBytes:  input.GetDisk().GetUsedBytes(),
			LimitBytes: input.GetDisk().GetLimitBytes(),
		},
		ObservedUnix: input.GetObservedUnix(),
	}
	if usage.CPU.UsedMillicores < 0 || usage.CPU.LimitMillicores < 0 ||
		usage.Memory.UsedBytes < 0 || usage.Memory.LimitBytes < 0 ||
		usage.Disk.UsedBytes < 0 || usage.Disk.LimitBytes < 0 ||
		usage.ObservedUnix < 0 {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata resource usage must not be negative")
	}
	return usage, nil
}

func runtimeMetadataBootStage(stage codespacev1.RuntimeBootStage) (string, bool) {
	switch stage {
	case codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_RUNTIME:
		return bootStagePrepareRuntime, true
	case codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_INITIALIZE_SYSTEM:
		return bootStageInitializeSystem, true
	case codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PREPARE_WORKSPACE:
		return bootStagePrepareWorkspace, true
	case codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_START_ENVIRONMENT:
		return bootStageStartEnvironment, true
	case codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY:
		return bootStagePublishReady, true
	case codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY:
		return bootStageReady, true
	default:
		return "", false
	}
}

func normalizeRuntimeMetadataLabel(label string) (string, error) {
	if !utf8.ValidString(label) {
		return "", errors.New("endpoint label must be valid UTF-8")
	}
	label = strings.TrimSpace(label)
	count := utf8.RuneCountInString(label)
	if count < 1 || count > 64 {
		return "", errors.New("endpoint label must be 1 to 64 characters")
	}
	for _, r := range label {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return "", errors.New("endpoint label contains invalid character")
		}
	}
	return label, nil
}

func validateRuntimeMetadataBoot(boot runtimeMetadataBoot) error {
	if boot.OperationRVersion <= 0 {
		return errors.New("boot operation_rversion must be positive")
	}
	if bootStageRank(boot.Stage) < 0 {
		return fmt.Errorf("invalid boot stage %q", boot.Stage)
	}
	if boot.StartedUnix <= 0 {
		return errors.New("boot started_unix must be positive")
	}
	if boot.LastUpdateUnix < boot.StartedUnix {
		return errors.New("boot last_update_unix must not precede started_unix")
	}
	return nil
}

func validateRuntimeMetadataState(codespace *codespace_model.Codespace, metadata runtimeMetadata) error {
	switch codespace.Status {
	case codespace_model.StatusCreating:
		if !currentOperationMatches(codespace, codespace_model.OperationCreate, metadata.Boot.OperationRVersion) {
			return ErrRuntimeMetadataStaleOperation
		}
	case codespace_model.StatusStopped:
		if !currentOperationMatches(codespace, codespace_model.OperationResume, metadata.Boot.OperationRVersion) {
			return ErrRuntimeMetadataStaleOperation
		}
	case codespace_model.StatusRunning:
		if metadata.Boot.Stage != bootStageReady || metadata.Boot.OperationRVersion > codespace.OperationRVersion {
			return ErrRuntimeMetadataStaleOperation
		}
	default:
		return ErrRuntimeMetadataStaleOperation
	}
	return nil
}

func validateRuntimeMetadataStageForward(current, next runtimeMetadata) error {
	if current.Boot.OperationRVersion != next.Boot.OperationRVersion {
		return nil
	}
	if bootStageRank(next.Boot.Stage) < bootStageRank(current.Boot.Stage) {
		return ErrRuntimeMetadataStaleOperation
	}
	return nil
}

func currentOperationMatches(codespace *codespace_model.Codespace, operationType string, operationRVersion int64) bool {
	return codespace.OperationRVersion == operationRVersion &&
		codespace.OperationType == operationType &&
		codespace.OperationStatus == codespace_model.OperationStatusRunning
}

func createOrResumeOperationActive(codespace *codespace_model.Codespace, now int64) bool {
	switch codespace.Status {
	case codespace_model.StatusCreating:
		return currentOperationMatches(codespace, codespace_model.OperationCreate, codespace.OperationRVersion) &&
			codespace.OperationDeadlineUnix > now
	case codespace_model.StatusStopped:
		return currentOperationMatches(codespace, codespace_model.OperationResume, codespace.OperationRVersion) &&
			codespace.OperationDeadlineUnix > now
	default:
		return false
	}
}

func bootStageRank(stage string) int {
	switch stage {
	case bootStagePrepareRuntime:
		return 0
	case bootStageInitializeSystem:
		return 1
	case bootStagePrepareWorkspace:
		return 2
	case bootStageStartEnvironment:
		return 3
	case bootStagePublishReady:
		return 4
	case bootStageReady:
		return 5
	default:
		return -1
	}
}

func getRuntimeMetadataEntry(codespaceUUID string) (runtimeMetadataCacheEntry, bool, error) {
	if cache.GetCache() == nil {
		return runtimeMetadataCacheEntry{}, false, errors.New("cache is not initialized")
	}
	entry := runtimeMetadataCacheEntry{}
	exists, getErr := cache.GetCache().GetJSON(runtimeMetadataCacheKey(codespaceUUID), &entry)
	if getErr != nil {
		return runtimeMetadataCacheEntry{}, false, getErr.ToError()
	}
	return entry, exists, nil
}

func putRuntimeMetadataEntry(codespaceUUID string, entry runtimeMetadataCacheEntry) error {
	if cache.GetCache() == nil {
		return errors.New("cache is not initialized")
	}
	// Retain metadata beyond one offline window so transient missed reports do not immediately remove usable endpoint state.
	return cache.GetCache().PutJSON(runtimeMetadataCacheKey(codespaceUUID), entry, int64((setting.Codespace.ManagerOfflineTimeout*2)/time.Second))
}

func runtimeMetadataCacheKey(codespaceUUID string) string {
	return "codespace:runtime-meta:" + codespaceUUID
}

func runtimeMetadataLockKey(codespaceUUID string) string {
	return "codespace_runtime_metadata_" + codespaceUUID
}
