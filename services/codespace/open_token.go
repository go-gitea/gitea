// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

var (
	errOpenTokenLoginRestricted = errors.New("codespace user login restricted")

	// ErrOpenEndpointUnavailable is returned when the current lifecycle cannot open an endpoint.
	ErrOpenEndpointUnavailable = errors.New("codespace endpoint is not currently available")
)

const (
	// OpenTokenDeniedInvalidCredentials means the submitted open code is invalid.
	OpenTokenDeniedInvalidCredentials = "invalid_credentials"
	// OpenTokenDeniedLoginRestricted means the Codespace creator cannot currently log in.
	OpenTokenDeniedLoginRestricted = "login_restricted"
	// OpenTokenDeniedCodespaceNotFound means the Codespace no longer exists.
	OpenTokenDeniedCodespaceNotFound = "codespace_not_found"
	// OpenTokenDeniedCodespaceNotRunning means the Codespace is not running.
	OpenTokenDeniedCodespaceNotRunning = "codespace_not_running"
	// OpenTokenDeniedManagerMismatch means the code or Codespace is bound to another Manager.
	OpenTokenDeniedManagerMismatch = "manager_mismatch"
	// OpenTokenDeniedPermissionDenied means the code no longer matches the Codespace creator.
	OpenTokenDeniedPermissionDenied = "permission_denied"
	// OpenTokenDeniedStateUnavailable means the lifecycle state cannot accept the open request.
	OpenTokenDeniedStateUnavailable = "state_unavailable"
	// OpenTokenDeniedMetadataRebuilding means Runtime Metadata is absent or not ready.
	OpenTokenDeniedMetadataRebuilding = "metadata_rebuilding"
	// OpenTokenDeniedEndpointNotFound means the authenticated Endpoint binding is no longer private.
	OpenTokenDeniedEndpointNotFound = "endpoint_not_found"
	// OpenTokenDeniedVersionExhausted means interaction_generation cannot advance.
	OpenTokenDeniedVersionExhausted = "version_exhausted"
)

// IssueOpenTokenOptions identifies one authenticated Gitea Web open request.
type IssueOpenTokenOptions struct {
	UserID        int64
	CodespaceUUID string
	EndpointID    string
}

// IssueOpenTokenResult contains the code and current interaction generation for a Gateway redirect.
type IssueOpenTokenResult struct {
	Code                  string
	ManagerID             int64
	InteractionGeneration int64
	RedirectURL           string
}

// OpenEndpointOptions identifies one authenticated Gitea Web open request.
type OpenEndpointOptions struct {
	UserID        int64
	CodespaceUUID string
	EndpointID    string
}

// OpenEndpointResult contains the redirect target produced for one Web open request.
type OpenEndpointResult struct {
	RedirectURL           string
	Public                bool
	InteractionGeneration int64
}

// OpenEndpointInfo describes the current Web open target without changing state.
type OpenEndpointInfo struct {
	CodespaceUUID        string
	EndpointID           string
	Label                string
	TargetURL            string
	Public               bool
	Available            bool
	NotAvailableCategory string
}

// OpenEndpoint redirects public Endpoints directly and private targets through Open Token.
func OpenEndpoint(ctx context.Context, opts OpenEndpointOptions) (*OpenEndpointResult, error) {
	info, err := InspectOpenEndpoint(ctx, opts)
	if err != nil {
		return nil, err
	}
	if !info.Available {
		return nil, fmt.Errorf("%w: %s", ErrOpenEndpointUnavailable, info.NotAvailableCategory)
	}
	if info.Public {
		return &OpenEndpointResult{
			RedirectURL: info.TargetURL,
			Public:      true,
		}, nil
	}
	issued, err := IssueOpenToken(ctx, IssueOpenTokenOptions(opts))
	if err != nil {
		return nil, err
	}
	return &OpenEndpointResult{
		RedirectURL:           issued.RedirectURL,
		InteractionGeneration: issued.InteractionGeneration,
	}, nil
}

// InspectOpenEndpoint returns the current Web open target without signing an Open Code.
func InspectOpenEndpoint(ctx context.Context, opts OpenEndpointOptions) (*OpenEndpointInfo, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	if err := validateOpenEndpointID(opts.EndpointID); err != nil {
		return nil, err
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if !setting.Codespace.Enabled {
		return unavailableOpenEndpoint(opts, "Codespace", OpenTokenDeniedStateUnavailable), nil
	}

	var result *OpenEndpointInfo
	err := globallock.LockAndDo(ctx, openTokenLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
		if err != nil {
			return err
		}
		if !has {
			return errors.New("codespace not found")
		}
		if codespace.UserID != opts.UserID {
			return errors.New("codespace user mismatch")
		}
		if codespace.Status != codespace_model.StatusRunning {
			result = unavailableOpenEndpoint(opts, "Codespace", OpenTokenDeniedCodespaceNotRunning)
			return nil
		}
		currentManager, err := loadFetchManager(ctx, codespace.ManagerID)
		if err != nil {
			return err
		}
		if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
			result = unavailableOpenEndpoint(opts, "Codespace", OpenTokenDeniedStateUnavailable)
			return nil
		}
		gatewayURL, err := loadManagerGatewayURL(ctx, codespace.ManagerID)
		if err != nil {
			return err
		}
		if err := checkCodespaceCreatorForOpen(ctx, codespace, opts.UserID); err != nil {
			return err
		}
		entry, hasEntry, err := getRuntimeMetadataEntry(opts.CodespaceUUID)
		if err != nil {
			return err
		}
		if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
			result = unavailableOpenEndpoint(opts, "Codespace", OpenTokenDeniedMetadataRebuilding)
			return nil
		}
		result, err = openEndpointInfo(codespace, entry.Metadata, gatewayURL, opts)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateOpenTokenOptions contains one Gateway authorization-code exchange.
type ValidateOpenTokenOptions struct {
	Code string
}

// ValidateOpenTokenResult contains the mutually exclusive open-code exchange result.
type ValidateOpenTokenResult struct {
	Allowed               bool
	DeniedCategory        string
	UserID                int64
	CodespaceUUID         string
	EndpointID            string
	InteractionGeneration int64
}

type openTokenCacheEntry struct {
	UserID        int64  `json:"user_id"`
	CodespaceUUID string `json:"codespace_uuid"`
	EndpointID    string `json:"endpoint_id"`
	ManagerID     int64  `json:"manager_id"`
	IssuedUnix    int64  `json:"issued_unix"`
	ExpiresUnix   int64  `json:"expires_unix"`
}

// IssueOpenToken issues a one-time Gateway Open Token after current Web access checks.
func IssueOpenToken(ctx context.Context, opts IssueOpenTokenOptions) (*IssueOpenTokenResult, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	if err := validateOpenEndpointID(opts.EndpointID); err != nil {
		return nil, err
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if !setting.Codespace.Enabled {
		return nil, ErrOpenEndpointUnavailable
	}
	code := generateOpenTokenCode()
	entryKey := openTokenCacheKey(code)
	var result *IssueOpenTokenResult

	err := globallock.LockAndDo(ctx, openTokenLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				return errors.New("codespace not found")
			}
			if codespace.UserID != opts.UserID {
				return errors.New("codespace user mismatch")
			}
			if codespace.Status != codespace_model.StatusRunning {
				return errors.New("codespace is not running")
			}
			if hasActiveOperation(codespace) && !isQueuedIdleStop(codespace) {
				return errors.New("codespace has active operation")
			}
			currentManager, err := loadFetchManager(ctx, codespace.ManagerID)
			if err != nil {
				return err
			}
			if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
				return errors.New("manager is not online")
			}
			gatewayURL, err := loadManagerGatewayURL(ctx, codespace.ManagerID)
			if err != nil {
				return err
			}
			if err := checkCodespaceCreatorForOpen(ctx, codespace, opts.UserID); err != nil {
				return err
			}
			entry, hasEntry, err := getRuntimeMetadataEntry(opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
				return errors.New("runtime metadata is not ready")
			}
			if opts.EndpointID != "workspace" && !privateEndpointExists(entry.Metadata, opts.EndpointID) {
				return errors.New("endpoint is not private")
			}

			now := time.Now().Unix()
			cacheEntry := openTokenCacheEntry{
				UserID:        opts.UserID,
				CodespaceUUID: opts.CodespaceUUID,
				EndpointID:    opts.EndpointID,
				ManagerID:     codespace.ManagerID,
				IssuedUnix:    now,
				ExpiresUnix:   now + int64(setting.Codespace.OpenTokenExpire/time.Second),
			}
			if err := putOpenTokenCacheEntry(entryKey, cacheEntry); err != nil {
				return err
			}
			redirectURL, err := gatewayOpenURL(gatewayURL, opts.CodespaceUUID, opts.EndpointID, code)
			if err != nil {
				_ = deleteOpenTokenCacheEntry(entryKey)
				return err
			}
			nextGeneration, err := advanceCodespaceInteraction(ctx, codespace, now)
			if err != nil {
				_ = deleteOpenTokenCacheEntry(entryKey)
				return err
			}
			result = &IssueOpenTokenResult{
				Code:                  code,
				ManagerID:             codespace.ManagerID,
				InteractionGeneration: nextGeneration,
				RedirectURL:           redirectURL,
			}
			return nil
		})
	})
	if err != nil {
		_ = deleteOpenTokenCacheEntry(entryKey)
		return nil, err
	}
	return result, nil
}

// ValidateOpenToken validates and consumes one Gateway Open Token.
func ValidateOpenToken(ctx context.Context, manager *codespace_model.Manager, opts ValidateOpenTokenOptions) (*ValidateOpenTokenResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if !setting.Codespace.Enabled {
		return denyOpenToken(OpenTokenDeniedStateUnavailable), nil
	}
	if !validOpenTokenCode(opts.Code) {
		return denyOpenToken(OpenTokenDeniedInvalidCredentials), nil
	}
	key := openTokenCacheKey(opts.Code)
	entry, hasEntry, badEntry, err := getOpenTokenCacheEntry(key)
	if err != nil {
		return nil, err
	}
	if badEntry {
		_ = deleteOpenTokenCacheEntry(key)
		return denyOpenToken(OpenTokenDeniedInvalidCredentials), nil
	}
	if !hasEntry {
		return denyOpenToken(OpenTokenDeniedInvalidCredentials), nil
	}

	var result *ValidateOpenTokenResult
	err = globallock.LockAndDo(ctx, openTokenLockKey(entry.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentEntry, hasEntry, badEntry, err := getOpenTokenCacheEntry(key)
			if err != nil {
				return err
			}
			if badEntry {
				_ = deleteOpenTokenCacheEntry(key)
				result = denyOpenToken(OpenTokenDeniedInvalidCredentials)
				return nil
			}
			if !hasEntry || currentEntry != entry {
				result = denyOpenToken(OpenTokenDeniedInvalidCredentials)
				return nil
			}
			now := time.Now().Unix()
			if now >= currentEntry.ExpiresUnix {
				_ = deleteOpenTokenCacheEntry(key)
				result = denyOpenToken(OpenTokenDeniedInvalidCredentials)
				return nil
			}
			if currentEntry.ManagerID != manager.ID {
				result = denyOpenToken(OpenTokenDeniedManagerMismatch)
				return nil
			}
			currentManager, err := loadFetchManager(ctx, manager.ID)
			if err != nil {
				return err
			}
			if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
				result = denyOpenToken(OpenTokenDeniedStateUnavailable)
				return nil
			}

			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(currentEntry.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				result = denyOpenToken(OpenTokenDeniedCodespaceNotFound)
				return nil
			}
			if codespace.ManagerID != manager.ID {
				result = denyOpenToken(OpenTokenDeniedManagerMismatch)
				return nil
			}
			if codespace.UserID != currentEntry.UserID {
				result = denyOpenToken(OpenTokenDeniedPermissionDenied)
				return nil
			}
			if codespace.Status != codespace_model.StatusRunning {
				result = denyOpenToken(OpenTokenDeniedCodespaceNotRunning)
				return nil
			}
			if hasActiveOperation(codespace) && !isQueuedIdleStop(codespace) {
				result = denyOpenToken(OpenTokenDeniedStateUnavailable)
				return nil
			}
			if err := checkCodespaceCreatorForOpen(ctx, codespace, currentEntry.UserID); err != nil {
				if user_model.IsErrUserNotExist(err) || errors.Is(err, errOpenTokenLoginRestricted) {
					result = denyOpenToken(OpenTokenDeniedLoginRestricted)
					return nil
				}
				return err
			}
			entry, hasEntry, err := getRuntimeMetadataEntry(currentEntry.CodespaceUUID)
			if err != nil {
				return err
			}
			if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
				result = denyOpenToken(OpenTokenDeniedMetadataRebuilding)
				return nil
			}
			if currentEntry.EndpointID != "workspace" && !privateEndpointExists(entry.Metadata, currentEntry.EndpointID) {
				result = denyOpenToken(OpenTokenDeniedEndpointNotFound)
				return nil
			}
			if err := deleteOpenTokenCacheEntry(key); err != nil {
				return err
			}
			nextGeneration, err := advanceCodespaceInteraction(ctx, codespace, now)
			if err != nil {
				if err == errInteractionVersionExhausted {
					result = denyOpenToken(OpenTokenDeniedVersionExhausted)
					return nil
				}
				return err
			}
			result = &ValidateOpenTokenResult{
				Allowed:               true,
				UserID:                currentEntry.UserID,
				CodespaceUUID:         currentEntry.CodespaceUUID,
				EndpointID:            currentEntry.EndpointID,
				InteractionGeneration: nextGeneration,
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func checkCodespaceCreatorForOpen(ctx context.Context, codespace *codespace_model.Codespace, userID int64) error {
	if userID != codespace.UserID {
		return errors.New("codespace user mismatch")
	}
	user, err := user_model.GetUserByID(ctx, codespace.UserID)
	if err != nil {
		return err
	}
	canUseGateway, err := userCanUseGatewayAccess(ctx, user)
	if err != nil {
		return err
	}
	if !canUseGateway {
		return errOpenTokenLoginRestricted
	}
	return nil
}

var errInteractionVersionExhausted = errors.New("interaction generation exhausted")

func advanceCodespaceInteraction(ctx context.Context, codespace *codespace_model.Codespace, now int64) (int64, error) {
	nextGeneration, err := codespace_model.NextVersion(codespace.InteractionGeneration)
	if err != nil {
		return 0, errInteractionVersionExhausted
	}
	codespace.InteractionGeneration = nextGeneration
	codespace.LastActiveUnix = now
	cols := []string{"interaction_generation", "last_active_unix"}
	if isQueuedIdleStop(codespace) {
		codespace.UpdatedUnix = now
		clearActiveOperation(codespace)
		cols = append(cols,
			"operation_type",
			"operation_status",
			"operation_trigger",
			"operation_created_unix",
			"operation_started_unix",
			"operation_deadline_unix",
			"updated_unix",
		)
	}
	if _, err := db.GetEngine(ctx).ID(codespace.UUID).Cols(cols...).Update(codespace); err != nil {
		return 0, err
	}
	return nextGeneration, nil
}

func openEndpointInfo(codespace *codespace_model.Codespace, metadata runtimeMetadata, gatewayURL string, opts OpenEndpointOptions) (*OpenEndpointInfo, error) {
	label := "Workspace"
	public := false
	found := opts.EndpointID == "workspace"
	for _, endpoint := range metadata.Endpoints {
		if endpoint.EndpointID != opts.EndpointID {
			continue
		}
		found = true
		label = endpoint.Label
		public = endpoint.Public
		break
	}
	if !found {
		return unavailableOpenEndpoint(opts, opts.EndpointID, OpenTokenDeniedEndpointNotFound), nil
	}
	targetURL, err := gatewayEndpointURL(gatewayURL, opts.CodespaceUUID, opts.EndpointID)
	if err != nil {
		return nil, err
	}
	info := &OpenEndpointInfo{
		CodespaceUUID: opts.CodespaceUUID,
		EndpointID:    opts.EndpointID,
		Label:         label,
		TargetURL:     targetURL,
		Public:        public,
		Available:     true,
	}
	if public {
		if hasActiveOperation(codespace) {
			info.Available = false
			info.NotAvailableCategory = OpenTokenDeniedStateUnavailable
		}
		return info, nil
	}
	if hasActiveOperation(codespace) && !isQueuedIdleStop(codespace) {
		info.Available = false
		info.NotAvailableCategory = OpenTokenDeniedStateUnavailable
	}
	return info, nil
}

func unavailableOpenEndpoint(opts OpenEndpointOptions, label, category string) *OpenEndpointInfo {
	return &OpenEndpointInfo{
		CodespaceUUID:        opts.CodespaceUUID,
		EndpointID:           opts.EndpointID,
		Label:                label,
		NotAvailableCategory: category,
	}
}

func validateOpenEndpointID(endpointID string) error {
	if endpointID == "workspace" || endpointIDPattern.MatchString(endpointID) {
		return nil
	}
	return errors.New("invalid endpoint_id")
}

func loadManagerGatewayURL(ctx context.Context, managerID int64) (string, error) {
	address := new(codespace_model.ManagerAddress)
	has, err := db.GetEngine(ctx).
		Where("manager_id = ? AND kind = ?", managerID, codespace_model.ManagerAddressGateway).
		Get(address)
	if err != nil {
		return "", err
	}
	if !has {
		return "", errors.New("manager gateway address not found")
	}
	return address.Address, nil
}

func gatewayOpenURL(rawGatewayURL, codespaceUUID, endpointID, code string) (string, error) {
	target, err := gatewayEndpointURL(rawGatewayURL, codespaceUUID, endpointID)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	parsed.Path = "/.gitea-codespace/open"
	values := parsed.Query()
	values.Set("code", code)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func gatewayEndpointURL(rawGatewayURL, codespaceUUID, endpointID string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawGatewayURL))
	if err != nil {
		return "", fmt.Errorf("parse gateway url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("gateway url must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("gateway url host is required")
	}
	uuid32, err := codespace_model.UUID32(codespaceUUID)
	if err != nil {
		return "", err
	}
	host := uuid32 + "." + parsed.Host
	if endpointID != "workspace" {
		if !endpointIDPattern.MatchString(endpointID) {
			return "", errors.New("invalid endpoint_id")
		}
		host = endpointID + "-" + host
	}
	target := &url.URL{
		Scheme: parsed.Scheme,
		Host:   host,
		Path:   "/",
	}
	return target.String(), nil
}

func generateOpenTokenCode() string {
	return hex.EncodeToString(util.CryptoRandomBytes(32))
}

func validOpenTokenCode(code string) bool {
	if len(code) != 64 {
		return false
	}
	_, err := hex.DecodeString(code)
	return err == nil
}

func openTokenCacheKey(code string) string {
	sum := sha256.Sum256([]byte(code))
	return "codespace:open-code:" + hex.EncodeToString(sum[:])
}

func putOpenTokenCacheEntry(key string, entry openTokenCacheEntry) error {
	if cache.GetCache() == nil {
		return errors.New("cache is not initialized")
	}
	return cache.GetCache().PutJSON(key, entry, int64(setting.Codespace.OpenTokenExpire/time.Second))
}

func getOpenTokenCacheEntry(key string) (openTokenCacheEntry, bool, bool, error) {
	if cache.GetCache() == nil {
		return openTokenCacheEntry{}, false, false, errors.New("cache is not initialized")
	}
	entry := openTokenCacheEntry{}
	exists, getErr := cache.GetCache().GetJSON(key, &entry)
	if getErr != nil {
		return openTokenCacheEntry{}, false, true, nil
	}
	return entry, exists, false, nil
}

func deleteOpenTokenCacheEntry(key string) error {
	if cache.GetCache() == nil {
		return errors.New("cache is not initialized")
	}
	return cache.GetCache().Delete(key)
}

func denyOpenToken(category string) *ValidateOpenTokenResult {
	return &ValidateOpenTokenResult{DeniedCategory: category}
}

func openTokenLockKey(codespaceUUID string) string {
	return codespaceInteractionLockKey(codespaceUUID)
}
