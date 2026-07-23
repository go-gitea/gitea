// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stdjson "encoding/json" //nolint:depguard // strict protocol decoding needs DisallowUnknownFields.
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
)

const (
	bootStagePrepareRuntime   = "prepare-runtime"
	bootStageInitializeSystem = "initialize-system"
	bootStagePrepareWorkspace = "prepare-workspace"
	bootStageStartEnvironment = "start-environment"
	bootStagePublishRuntime   = "publish-runtime"
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
	MetadataJSON       string
	MetadataGeneration int64
}

type runtimeMetadataCacheEntry struct {
	Metadata         runtimeMetadata `json:"metadata"`
	Generation       int64           `json:"generation"`
	ContentHash      string          `json:"content_hash"`
	LastReportedUnix int64           `json:"last_reported_unix"`
}

type runtimeMetadata struct {
	Endpoints []runtimeMetadataEndpoint `json:"endpoints"`
	Boot      runtimeMetadataBoot       `json:"boot"`
}

type runtimeMetadataEndpoint struct {
	EndpointID string `json:"endpoint_id"`
	Label      string `json:"label"`
	Public     bool   `json:"public"`
}

type runtimeMetadataBoot struct {
	OperationRVersion int64  `json:"operation_rversion"`
	Stage             string `json:"stage"`
	StartedUnix       int64  `json:"started_unix"`
	LastUpdateUnix    int64  `json:"last_update_unix"`
}

type rawRuntimeMetadata struct {
	Endpoints *[]rawRuntimeMetadataEndpoint `json:"endpoints"`
	Boot      *runtimeMetadataBoot          `json:"boot"`
}

type rawRuntimeMetadataEndpoint struct {
	EndpointID string `json:"endpoint_id"`
	Label      string `json:"label"`
	Public     *bool  `json:"public"`
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
	metadata, contentHash, err := normalizeRuntimeMetadata(opts.MetadataJSON)
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

func normalizeRuntimeMetadata(raw string) (runtimeMetadata, string, error) {
	if raw == "" {
		return runtimeMetadata{}, "", errors.New("metadata_json is required")
	}
	if !utf8.ValidString(raw) {
		return runtimeMetadata{}, "", errors.New("metadata_json must be valid UTF-8")
	}
	if int64(len(raw)) > setting.Codespace.RuntimeMetadataMaxSize {
		return runtimeMetadata{}, "", errors.New("metadata_json exceeds maximum size")
	}
	var input rawRuntimeMetadata
	decoder := stdjson.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return runtimeMetadata{}, "", fmt.Errorf("decode runtime metadata: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return runtimeMetadata{}, "", errors.New("decode runtime metadata: trailing content is not allowed")
	}
	if input.Endpoints == nil {
		return runtimeMetadata{}, "", errors.New("runtime metadata endpoints is required")
	}
	if input.Boot == nil {
		return runtimeMetadata{}, "", errors.New("runtime metadata boot is required")
	}

	metadata := runtimeMetadata{
		Endpoints: make([]runtimeMetadataEndpoint, 0, len(*input.Endpoints)),
		Boot:      *input.Boot,
	}
	if err := validateRuntimeMetadataBoot(metadata.Boot); err != nil {
		return runtimeMetadata{}, "", err
	}
	if len(*input.Endpoints) > 64 {
		return runtimeMetadata{}, "", errors.New("runtime metadata endpoints exceed 64")
	}
	seen := make(map[string]struct{}, len(*input.Endpoints))
	for _, endpoint := range *input.Endpoints {
		if endpoint.Public == nil {
			return runtimeMetadata{}, "", errors.New("endpoint public is required")
		}
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

func normalizeRuntimeMetadataEndpoint(endpoint rawRuntimeMetadataEndpoint) (runtimeMetadataEndpoint, error) {
	if !endpointIDPattern.MatchString(endpoint.EndpointID) {
		return runtimeMetadataEndpoint{}, fmt.Errorf("invalid endpoint_id %q", endpoint.EndpointID)
	}
	label, err := normalizeRuntimeMetadataLabel(endpoint.Label)
	if err != nil {
		return runtimeMetadataEndpoint{}, err
	}
	if endpoint.EndpointID == "workspace" && *endpoint.Public {
		return runtimeMetadataEndpoint{}, errors.New("workspace endpoint must be private")
	}
	return runtimeMetadataEndpoint{
		EndpointID: endpoint.EndpointID,
		Label:      label,
		Public:     *endpoint.Public,
	}, nil
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
	case bootStagePublishRuntime:
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
	return cache.GetCache().PutJSON(runtimeMetadataCacheKey(codespaceUUID), entry, int64((setting.Codespace.ManagerOfflineTimeout*2)/time.Second))
}

func runtimeMetadataCacheKey(codespaceUUID string) string {
	return "codespace:runtime-meta:" + codespaceUUID
}

func runtimeMetadataLockKey(codespaceUUID string) string {
	return "codespace_runtime_metadata_" + codespaceUUID
}
