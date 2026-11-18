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
	"net"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
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
	"gitea.dev/modules/util"

	"google.golang.org/protobuf/proto"
)

const workspaceEndpointID = "workspace"

var (
	endpointIDPattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,28}[a-z0-9])?$`)
	runtimeSSHUserPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$`)
)

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
	CodespaceID        int64
	Metadata           *codespacev1.RuntimeMetadata
	MetadataGeneration int64
}

type runtimeMetadataCacheEntry struct {
	Metadata    runtimeMetadata `json:"metadata"`
	Generation  int64           `json:"generation"`
	ContentHash string          `json:"content_hash"`
}

type runtimeMetadata struct {
	Endpoints       []runtimeMetadataEndpoint    `json:"endpoints"`
	Boot            runtimeMetadataBoot          `json:"boot"`
	ResourceUsage   runtimeMetadataResourceUsage `json:"resource_usage"`
	WorkspaceFolder string                       `json:"workspace_folder"`
	SSH             string                       `json:"ssh"`
}

type runtimeMetadataEndpoint struct {
	EndpointID string `json:"endpoint_id"`
	Label      string `json:"label"`
	Public     bool   `json:"public"`
	Port       uint32 `json:"port,omitempty"`
	URL        string `json:"url"`
}

func (m runtimeMetadata) endpointByID(endpointID string) (runtimeMetadataEndpoint, bool) {
	for _, endpoint := range m.Endpoints {
		if endpoint.EndpointID == endpointID {
			return endpoint, true
		}
	}
	return runtimeMetadataEndpoint{}, false
}

func runtimeMetadataReadyForAccess(codespace *codespace_model.Codespace, metadata runtimeMetadata) bool {
	if metadata.Boot.Stage != codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY {
		return false
	}
	if hasActiveStartup(codespace, time.Now().Unix()) {
		return metadata.Boot.OperationRVersion == codespace.OperationRVersion
	}
	return codespace.Status == codespace_model.StatusRunning && (!hasActiveOperation(codespace) || isQueuedIdleStop(codespace)) && metadata.Boot.OperationRVersion == codespace.RuntimeOperationRVersion
}

type runtimeMetadataBoot struct {
	OperationRVersion int64                        `json:"operation_rversion"`
	Stage             codespacev1.RuntimeBootStage `json:"stage"`
	StartedUnix       int64                        `json:"started_unix"`
	LastUpdateUnix    int64                        `json:"last_update_unix"`
}

type runtimeMetadataResourceUsage struct {
	CPU          runtimeMetadataCPUUsage   `json:"cpu"`
	Memory       runtimeMetadataBytesUsage `json:"memory"`
	Disk         runtimeMetadataBytesUsage `json:"disk"`
	ObservedUnix int64                     `json:"observed_unix"`
}

type runtimeMetadataCPUUsage struct {
	UsedMillicores  int64 `json:"used_millicores"`
	LimitMillicores int64 `json:"limit_millicores"`
}

type runtimeMetadataBytesUsage struct {
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
	if opts.CodespaceID <= 0 {
		return util.NewInvalidArgumentErrorf("codespace_id must be positive")
	}
	if opts.MetadataGeneration <= 0 {
		return util.NewInvalidArgumentErrorf("metadata_generation must be positive")
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
		return util.NewInvalidArgumentErrorf("%v", err)
	}
	if err := validateRuntimeAddresses(metadata, manager); err != nil {
		return util.NewInvalidArgumentErrorf("%v", err)
	}

	return globallock.LockAndDo(ctx, runtimeMetadataLockKey(opts.CodespaceID), func(ctx context.Context) error {
		allowed, err = currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRuntimeMetadataManagerOffline
		}
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Select(codespaceStateColumns).Get(codespace)
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

		current, hasCurrent, err := getRuntimeMetadataEntry(opts.CodespaceID)
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
			if metadata.Boot.OperationRVersion == current.Metadata.Boot.OperationRVersion {
				if metadata.Boot.Stage < current.Metadata.Boot.Stage {
					return ErrRuntimeMetadataStaleOperation
				}
				if metadata.ResourceUsage.ObservedUnix <= current.Metadata.ResourceUsage.ObservedUnix {
					metadata.ResourceUsage = current.Metadata.ResourceUsage
				}
			}
		}

		return putRuntimeMetadataEntry(opts.CodespaceID, runtimeMetadataCacheEntry{
			Metadata:    metadata,
			Generation:  opts.MetadataGeneration,
			ContentHash: contentHash,
		})
	})
}

func deleteRuntimeMetadata(codespaceID int64) {
	if cache.GetCache() == nil {
		return
	}
	_ = cache.GetCache().Delete(runtimeMetadataCacheKey(codespaceID))
}

func normalizeRuntimeMetadata(input *codespacev1.RuntimeMetadata) (runtimeMetadata, string, error) {
	if input == nil {
		return runtimeMetadata{}, "", errors.New("runtime metadata is required")
	}
	if int64(proto.Size(input)) > runtimeMetadataMaxSize {
		return runtimeMetadata{}, "", errors.New("runtime metadata exceeds maximum size")
	}
	if input.GetBoot() == nil {
		return runtimeMetadata{}, "", errors.New("runtime metadata boot is required")
	}
	resourceUsage, err := normalizeRuntimeMetadataResourceUsage(input.GetResourceUsage())
	if err != nil {
		return runtimeMetadata{}, "", err
	}
	bootStage := input.GetBoot().GetStage()

	workspaceFolder := input.GetWorkspaceFolder()
	if workspaceFolder != "" {
		if len(workspaceFolder) > 4096 || !utf8.ValidString(workspaceFolder) || strings.TrimSpace(workspaceFolder) != workspaceFolder || !path.IsAbs(workspaceFolder) || path.Clean(workspaceFolder) != workspaceFolder || strings.ContainsFunc(workspaceFolder, unicode.IsControl) {
			return runtimeMetadata{}, "", errors.New("runtime metadata workspace folder must be a clean absolute POSIX path")
		}
	} else if bootStage == codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_PUBLISH_READY || bootStage == codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY {
		return runtimeMetadata{}, "", errors.New("runtime metadata workspace folder is required when publishing ready state")
	}

	metadata := runtimeMetadata{
		Endpoints: make([]runtimeMetadataEndpoint, 0, len(input.GetEndpoints())),
		Boot: runtimeMetadataBoot{
			OperationRVersion: input.GetBoot().GetOperationRversion(),
			Stage:             bootStage,
			StartedUnix:       input.GetBoot().GetStartedUnix(),
			LastUpdateUnix:    input.GetBoot().GetLastUpdateUnix(),
		},
		ResourceUsage:   resourceUsage,
		WorkspaceFolder: workspaceFolder,
		SSH:             input.GetSsh(),
	}
	if err := validateRuntimeMetadataBoot(metadata.Boot); err != nil {
		return runtimeMetadata{}, "", err
	}
	if len(input.GetEndpoints()) > 64 {
		return runtimeMetadata{}, "", errors.New("runtime metadata endpoints exceed 64")
	}
	seen := make(map[string]struct{}, len(input.GetEndpoints()))
	for _, endpoint := range input.GetEndpoints() {
		normalized, err := normalizeRuntimeMetadataEndpoint(endpoint)
		if err != nil {
			return runtimeMetadata{}, "", err
		}
		if _, ok := seen[normalized.EndpointID]; ok {
			return runtimeMetadata{}, "", fmt.Errorf("duplicate endpoint_id %q", normalized.EndpointID)
		}
		seen[normalized.EndpointID] = struct{}{}
		metadata.Endpoints = append(metadata.Endpoints, normalized)
	}
	slices.SortFunc(metadata.Endpoints, func(a, b runtimeMetadataEndpoint) int {
		return strings.Compare(a.EndpointID, b.EndpointID)
	})

	// Samples can refresh without changing the endpoint and boot publication.
	publication := metadata
	publication.ResourceUsage = runtimeMetadataResourceUsage{}
	canonical, err := json.Marshal(publication)
	if err != nil {
		return runtimeMetadata{}, "", fmt.Errorf("encode canonical runtime metadata: %w", err)
	}
	if int64(len(canonical)) > runtimeMetadataMaxSize {
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
	normalized := runtimeMetadataEndpoint{
		EndpointID: endpoint.GetEndpointId(),
		Public:     endpoint.GetPublic(),
		Port:       endpoint.GetPort(),
		URL:        endpoint.GetUrl(),
	}
	if normalized.Port > 65535 {
		return runtimeMetadataEndpoint{}, fmt.Errorf("invalid endpoint port %d", normalized.Port)
	}
	if normalized.EndpointID == workspaceEndpointID {
		if normalized.Public || normalized.Port != 0 {
			return runtimeMetadataEndpoint{}, errors.New("runtime metadata workspace endpoint is invalid")
		}
		return normalized, nil
	}
	label := endpoint.GetLabel()
	if !utf8.ValidString(label) {
		return runtimeMetadataEndpoint{}, errors.New("endpoint label must be valid UTF-8")
	}
	label = strings.TrimSpace(label)
	if count := utf8.RuneCountInString(label); count < 1 || count > 64 {
		return runtimeMetadataEndpoint{}, errors.New("endpoint label must be 1 to 64 characters")
	}
	for _, r := range label {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return runtimeMetadataEndpoint{}, errors.New("endpoint label contains invalid character")
		}
	}
	normalized.Label = label
	return normalized, nil
}

func normalizeRuntimeMetadataResourceUsage(input *codespacev1.RuntimeResourceUsage) (runtimeMetadataResourceUsage, error) {
	if input == nil {
		return runtimeMetadataResourceUsage{}, nil
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
		Memory: runtimeMetadataBytesUsage{
			UsedBytes:  input.GetMemory().GetUsedBytes(),
			LimitBytes: input.GetMemory().GetLimitBytes(),
		},
		Disk: runtimeMetadataBytesUsage{
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
	if usage.Disk.LimitBytes == 0 {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata disk limit must be positive")
	}
	if usage.ObservedUnix > time.Now().Unix()+30 {
		return runtimeMetadataResourceUsage{}, errors.New("runtime metadata observation time is too far in the future")
	}
	return usage, nil
}

func validateRuntimeMetadataBoot(boot runtimeMetadataBoot) error {
	if boot.OperationRVersion <= 0 {
		return errors.New("boot operation_rversion must be positive")
	}
	if _, ok := codespacev1.RuntimeBootStage_name[int32(boot.Stage)]; !ok || boot.Stage == codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_UNSPECIFIED {
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
		if metadata.Boot.Stage != codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY || metadata.Boot.OperationRVersion != codespace.RuntimeOperationRVersion {
			return ErrRuntimeMetadataStaleOperation
		}
	default:
		return ErrRuntimeMetadataStaleOperation
	}
	return nil
}

func currentOperationMatches(codespace *codespace_model.Codespace, operationType string, operationRVersion int64) bool {
	return codespace.OperationRVersion == operationRVersion &&
		codespace_model.ActiveOperationType(codespace) == operationType &&
		codespace_model.IsOperationRunning(codespace)
}

func getRuntimeMetadataEntry(codespaceID int64) (runtimeMetadataCacheEntry, bool, error) {
	if cache.GetCache() == nil {
		return runtimeMetadataCacheEntry{}, false, errors.New("cache is not initialized")
	}
	entry := runtimeMetadataCacheEntry{}
	exists, getErr := cache.GetCache().GetJSON(runtimeMetadataCacheKey(codespaceID), &entry)
	if getErr != nil {
		return runtimeMetadataCacheEntry{}, false, getErr.ToError()
	}
	return entry, exists, nil
}

func putRuntimeMetadataEntry(codespaceID int64, entry runtimeMetadataCacheEntry) error {
	if cache.GetCache() == nil {
		return errors.New("cache is not initialized")
	}
	// Retain metadata beyond one offline window so transient missed reports do not immediately remove usable endpoint state.
	return cache.GetCache().PutJSON(runtimeMetadataCacheKey(codespaceID), entry, int64(runtimeMetadataMaxAge/time.Second))
}

func runtimeMetadataCacheKey(codespaceID int64) string {
	return "codespace:runtime-meta:" + strconv.FormatInt(codespaceID, 10)
}

func runtimeMetadataLockKey(codespaceID int64) string {
	return "codespace_runtime_metadata_" + strconv.FormatInt(codespaceID, 10)
}

func validateRuntimeAddresses(metadata runtimeMetadata, manager *codespace_model.Manager) error {
	for _, endpoint := range metadata.Endpoints {
		if _, err := parseRuntimeEndpointURL(endpoint.URL, manager.GatewayURL); err != nil {
			return err
		}
	}
	if metadata.SSH == "" {
		return nil
	}
	ssh, err := parseRuntimeSSHURI(metadata.SSH)
	if err != nil {
		return err
	}
	if ssh.Host != manager.GatewaySSHAddr {
		return errors.New("runtime SSH address must belong to the declared gateway")
	}
	return nil
}

func parseRuntimeEndpointURL(raw, gatewayURL string) (*url.URL, error) {
	gateway, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, err
	}
	address, err := url.Parse(raw)
	if err != nil || (address.Scheme != "http" && address.Scheme != "https") || address.User != nil || address.Fragment != "" ||
		validateDNSHost(address.Hostname()) != nil ||
		address.Scheme != gateway.Scheme || address.Port() != gateway.Port() ||
		!strings.HasSuffix(strings.ToLower(address.Hostname()), "."+strings.ToLower(gateway.Hostname())) {
		return nil, errors.New("runtime endpoint address must belong to the declared gateway")
	}
	return address, nil
}

func parseRuntimeSSHURI(raw string) (*url.URL, error) {
	address, err := url.Parse(raw)
	if err != nil || address.Scheme != "ssh" || address.User == nil || address.Opaque != "" || address.Path != "" || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" {
		return nil, errors.New("invalid runtime SSH URI")
	}
	host, port, err := net.SplitHostPort(address.Host)
	if err != nil || host == "" {
		return nil, errors.New("runtime SSH URI must include a host and port")
	}
	number, err := strconv.ParseUint(port, 10, 16)
	_, password := address.User.Password()
	if err != nil || number == 0 || password || !runtimeSSHUserPattern.MatchString(address.User.Username()) {
		return nil, errors.New("invalid runtime SSH user or port")
	}
	return address, nil
}
