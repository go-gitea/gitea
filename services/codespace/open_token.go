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

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
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

	// ErrOpenEndpointNotFound is returned when the Codespace is not visible to the requesting user.
	ErrOpenEndpointNotFound = errors.New("codespace not found")
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

// OpenEndpointOptions identifies one authenticated Gitea Web open request.
type OpenEndpointOptions struct {
	UserID      int64
	CodespaceID int64
	EndpointID  string
}

// OpenEndpointResult contains the redirect target produced for one Web open request.
type OpenEndpointResult struct {
	RedirectURL           string
	Public                bool
	InteractionGeneration int64
}

type openEndpointResult struct {
	redirectURL           string
	public                bool
	interactionGeneration int64
	code                  string
	managerID             int64
}

type openEndpointTarget struct {
	redirectURL         string
	public              bool
	available           bool
	unavailableCategory string
}

// OpenEndpoint redirects public Endpoints directly and private targets through Open Token.
func OpenEndpoint(ctx context.Context, opts OpenEndpointOptions) (*OpenEndpointResult, error) {
	opened, err := openEndpoint(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &OpenEndpointResult{
		RedirectURL:           opened.redirectURL,
		Public:                opened.public,
		InteractionGeneration: opened.interactionGeneration,
	}, nil
}

func openEndpoint(ctx context.Context, opts OpenEndpointOptions) (*openEndpointResult, error) {
	if opts.UserID <= 0 {
		return nil, errors.New("user_id must be positive")
	}
	if err := validateOpenEndpointID(opts.EndpointID); err != nil {
		return nil, err
	}
	if opts.CodespaceID <= 0 {
		return nil, errors.New("codespace_id must be positive")
	}
	if !setting.Codespace.Enabled {
		return nil, fmt.Errorf("%w: %s", ErrOpenEndpointUnavailable, OpenTokenDeniedStateUnavailable)
	}

	var result *openEndpointResult
	var unavailableCategory string
	var tokenCacheKey string
	err := globallock.LockAndDo(ctx, codespaceRowLockKey(opts.CodespaceID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				return ErrOpenEndpointNotFound
			}
			if codespace.UserID != opts.UserID {
				return ErrOpenEndpointNotFound
			}
			if codespace.UUID == "" {
				unavailableCategory = OpenTokenDeniedMetadataRebuilding
				return nil
			}
			if codespace.Status != codespace_model.StatusRunning {
				unavailableCategory = OpenTokenDeniedCodespaceNotRunning
				return nil
			}
			manager, err := loadCodespaceManager(ctx, codespace.ManagerID)
			if err != nil {
				return err
			}
			if manager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(manager) {
				unavailableCategory = OpenTokenDeniedStateUnavailable
				return nil
			}
			gatewayURL, err := loadManagerGatewayURL(ctx, codespace.ManagerID)
			if err != nil {
				return err
			}
			if err := checkCodespaceCreatorForOpen(ctx, codespace, opts.UserID); err != nil {
				return err
			}
			entry, hasEntry, err := getRuntimeMetadataEntry(codespace.UUID)
			if err != nil {
				return err
			}
			if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
				unavailableCategory = OpenTokenDeniedMetadataRebuilding
				return nil
			}
			target, err := openEndpointInfo(codespace, entry.Metadata, gatewayURL, opts)
			if err != nil {
				return err
			}
			if !target.available {
				unavailableCategory = target.unavailableCategory
				return nil
			}
			if target.public {
				result = &openEndpointResult{redirectURL: target.redirectURL, public: true}
				return nil
			}

			code := generateOpenTokenCode()
			tokenCacheKey = openTokenCacheKey(code)
			now := time.Now().Unix()
			if err := putOpenTokenCacheEntry(tokenCacheKey, openTokenCacheEntry{
				UserID:        opts.UserID,
				CodespaceUUID: codespace.UUID,
				EndpointID:    opts.EndpointID,
				ManagerID:     codespace.ManagerID,
				IssuedUnix:    now,
				ExpiresUnix:   now + int64(openTokenExpire/time.Second),
			}); err != nil {
				return err
			}
			redirectURL, err := gatewayOpenURL(gatewayURL, codespace.UUID, opts.EndpointID, code)
			if err != nil {
				return err
			}
			nextGeneration, err := advanceCodespaceInteraction(ctx, codespace, now)
			if err != nil {
				return err
			}
			result = &openEndpointResult{
				redirectURL:           redirectURL,
				interactionGeneration: nextGeneration,
				code:                  code,
				managerID:             codespace.ManagerID,
			}
			return nil
		})
	})
	if err != nil {
		if tokenCacheKey != "" {
			_ = deleteOpenTokenCacheEntry(tokenCacheKey)
		}
		return nil, err
	}
	if unavailableCategory != "" {
		return nil, fmt.Errorf("%w: %s", ErrOpenEndpointUnavailable, unavailableCategory)
	}
	return result, nil
}

// ValidateOpenTokenOptions contains one Gateway authorization-code exchange.
type ValidateOpenTokenOptions struct {
	Code string
}

type openTokenCacheEntry struct {
	UserID        int64  `json:"user_id"`
	CodespaceUUID string `json:"codespace_uuid"`
	EndpointID    string `json:"endpoint_id"`
	ManagerID     int64  `json:"manager_id"`
	IssuedUnix    int64  `json:"issued_unix"`
	ExpiresUnix   int64  `json:"expires_unix"`
}

// ValidateOpenToken validates and consumes one Gateway Open Token.
func ValidateOpenToken(ctx context.Context, manager *codespace_model.Manager, opts ValidateOpenTokenOptions) (*codespacev1.ValidateOpenTokenResponse, error) {
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

	var result *codespacev1.ValidateOpenTokenResponse
	err = globallock.LockAndDo(ctx, codespaceStateLockKey(entry.CodespaceUUID), func(ctx context.Context) error {
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
			currentManager, err := loadCodespaceManager(ctx, manager.ID)
			if err != nil {
				return err
			}
			if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
				result = denyOpenToken(OpenTokenDeniedStateUnavailable)
				return nil
			}

			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).Where("uuid = ?", currentEntry.CodespaceUUID).Get(codespace)
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
			endpoint, found := entry.Metadata.endpointByID(currentEntry.EndpointID)
			if !found || endpoint.Public {
				result = denyOpenToken(OpenTokenDeniedEndpointNotFound)
				return nil
			}
			// Consume the code before granting access so concurrent Gateway exchanges remain single-use.
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
			result = &codespacev1.ValidateOpenTokenResponse{
				Outcome: &codespacev1.ValidateOpenTokenResponse_Allowed{
					Allowed: &codespacev1.OpenTokenBinding{
						UserId:                currentEntry.UserID,
						RuntimeUuid:           currentEntry.CodespaceUUID,
						EndpointId:            currentEntry.EndpointID,
						InteractionGeneration: nextGeneration,
					},
				},
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
	canUseGateway, err := codespaceUserCanLogIn(ctx, user)
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
		// User activity cancels only a queued idle stop; explicit and already-running lifecycle operations retain ownership.
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
	if _, err := db.GetEngine(ctx).ID(codespace.ID).Cols(cols...).Update(codespace); err != nil {
		return 0, err
	}
	return nextGeneration, nil
}

func openEndpointInfo(codespace *codespace_model.Codespace, metadata runtimeMetadata, gatewayURL string, opts OpenEndpointOptions) (*openEndpointTarget, error) {
	endpoint, found := metadata.endpointByID(opts.EndpointID)
	if !found {
		return unavailableOpenEndpoint(OpenTokenDeniedEndpointNotFound), nil
	}
	targetURL, err := gatewayEndpointURL(gatewayURL, codespace.UUID, opts.EndpointID)
	if err != nil {
		return nil, err
	}
	target := &openEndpointTarget{
		redirectURL: targetURL,
		public:      endpoint.Public,
		available:   true,
	}
	if endpoint.Public {
		if hasActiveOperation(codespace) {
			target.available = false
			target.unavailableCategory = OpenTokenDeniedStateUnavailable
		}
		return target, nil
	}
	if hasActiveOperation(codespace) && !isQueuedIdleStop(codespace) {
		target.available = false
		target.unavailableCategory = OpenTokenDeniedStateUnavailable
	}
	return target, nil
}

func unavailableOpenEndpoint(category string) *openEndpointTarget {
	return &openEndpointTarget{
		unavailableCategory: category,
	}
}

func validateOpenEndpointID(endpointID string) error {
	if endpointID == workspaceEndpointID || endpointIDPattern.MatchString(endpointID) {
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
	if endpointID != workspaceEndpointID {
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
	return cache.GetCache().PutJSON(key, entry, int64(openTokenExpire/time.Second))
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

func denyOpenToken(category string) *codespacev1.ValidateOpenTokenResponse {
	return &codespacev1.ValidateOpenTokenResponse{
		Outcome: &codespacev1.ValidateOpenTokenResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: category},
		},
	}
}
