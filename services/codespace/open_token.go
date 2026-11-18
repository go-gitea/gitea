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
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
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
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace, has, err := findCreatorCodespace(ctx, opts.UserID, opts.CodespaceID)
		if err != nil {
			return err
		}
		if !has {
			return ErrOpenEndpointNotFound
		}
		if codespace.Status != codespace_model.StatusRunning && !hasActiveStartup(codespace, time.Now().Unix()) {
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
		if err := checkCodespaceCreatorForOpen(ctx, codespace, opts.UserID); err != nil {
			return err
		}
		entry, hasEntry, err := getRuntimeMetadataEntry(codespace.ID)
		if err != nil {
			return err
		}
		if !hasEntry || !runtimeMetadataReadyForAccess(codespace, entry.Metadata) {
			unavailableCategory = OpenTokenDeniedMetadataRebuilding
			return nil
		}
		endpoint, found := entry.Metadata.endpointByID(opts.EndpointID)
		if !found {
			unavailableCategory = OpenTokenDeniedEndpointNotFound
			return nil
		}
		redirectURL, err := parseRuntimeEndpointURL(endpoint.URL, manager.GatewayURL)
		if err != nil {
			unavailableCategory = OpenTokenDeniedMetadataRebuilding
			return nil
		}
		if hasActiveOperation(codespace) && !hasActiveStartup(codespace, time.Now().Unix()) && (endpoint.Public || !isQueuedIdleStop(codespace)) {
			unavailableCategory = OpenTokenDeniedStateUnavailable
			return nil
		}
		if endpoint.Public {
			result = &openEndpointResult{redirectURL: endpoint.URL, public: true}
			return nil
		}

		code := generateOpenTokenCode()
		now := time.Now().Unix()
		if err := db.Insert(ctx, &codespace_model.OpenToken{
			CodeHash:    openTokenCodeHash(code),
			CodespaceID: codespace.ID,
			UserID:      opts.UserID,
			EndpointID:  opts.EndpointID,
			ManagerID:   codespace.ManagerID,
			ExpiresUnix: now + int64(openTokenExpire/time.Second),
		}); err != nil {
			return err
		}
		redirectURL.Path = "/.gitea-codespace/open"
		redirectURL.RawPath = ""
		redirectURL.RawQuery = url.Values{"code": {code}}.Encode()
		nextGeneration, err := advanceCodespaceInteraction(ctx, codespace, now)
		if err != nil {
			return err
		}
		result = &openEndpointResult{
			redirectURL:           redirectURL.String(),
			interactionGeneration: nextGeneration,
			code:                  code,
			managerID:             codespace.ManagerID,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errCodespaceStateChanged) {
			return nil, fmt.Errorf("%w: %s", ErrOpenEndpointUnavailable, OpenTokenDeniedStateUnavailable)
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
	var result *codespacev1.ValidateOpenTokenResponse
	err := db.WithTx(ctx, func(ctx context.Context) error {
		openToken := new(codespace_model.OpenToken)
		hasEntry, err := db.GetEngine(ctx).ID(openTokenCodeHash(opts.Code)).Get(openToken)
		if err != nil {
			return err
		}
		if !hasEntry {
			result = denyOpenToken(OpenTokenDeniedInvalidCredentials)
			return nil
		}
		now := time.Now().Unix()
		if now >= openToken.ExpiresUnix {
			_, _ = db.GetEngine(ctx).ID(openToken.CodeHash).Delete(new(codespace_model.OpenToken))
			result = denyOpenToken(OpenTokenDeniedInvalidCredentials)
			return nil
		}
		if openToken.ManagerID != manager.ID {
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
		has, err := db.GetEngine(ctx).ID(openToken.CodespaceID).Omit("secrets_encrypted").Get(codespace)
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
		if codespace.UserID != openToken.UserID {
			result = denyOpenToken(OpenTokenDeniedPermissionDenied)
			return nil
		}
		if codespace.Status != codespace_model.StatusRunning && !hasActiveStartup(codespace, time.Now().Unix()) {
			result = denyOpenToken(OpenTokenDeniedCodespaceNotRunning)
			return nil
		}
		if hasActiveOperation(codespace) && !hasActiveStartup(codespace, time.Now().Unix()) && !isQueuedIdleStop(codespace) {
			result = denyOpenToken(OpenTokenDeniedStateUnavailable)
			return nil
		}
		if err := checkCodespaceCreatorForOpen(ctx, codespace, openToken.UserID); err != nil {
			if user_model.IsErrUserNotExist(err) || errors.Is(err, errOpenTokenLoginRestricted) {
				result = denyOpenToken(OpenTokenDeniedLoginRestricted)
				return nil
			}
			return err
		}
		entry, hasEntry, err := getRuntimeMetadataEntry(codespace.ID)
		if err != nil {
			return err
		}
		if !hasEntry || !runtimeMetadataReadyForAccess(codespace, entry.Metadata) {
			result = denyOpenToken(OpenTokenDeniedMetadataRebuilding)
			return nil
		}
		endpoint, found := entry.Metadata.endpointByID(openToken.EndpointID)
		if !found || endpoint.Public {
			result = denyOpenToken(OpenTokenDeniedEndpointNotFound)
			return nil
		}
		// Consume the code before granting access so concurrent Gateway exchanges remain single-use.
		deleted, err := db.GetEngine(ctx).ID(openToken.CodeHash).Delete(new(codespace_model.OpenToken))
		if err != nil {
			return err
		}
		if deleted != 1 {
			result = denyOpenToken(OpenTokenDeniedInvalidCredentials)
			return nil
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
					UserId:                openToken.UserID,
					CodespaceId:           codespace.ID,
					EndpointId:            openToken.EndpointID,
					InteractionGeneration: nextGeneration,
				},
			},
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errCodespaceStateChanged) {
			return denyOpenToken(OpenTokenDeniedStateUnavailable), nil
		}
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
	expected := snapshotCodespaceState(codespace)
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
			"operation_trigger",
			"operation_created_unix",
			"operation_started_unix",
			"operation_deadline_unix",
			"updated_unix",
		)
	}
	if err := updateCodespaceInteractionIfCurrent(ctx, expected, codespace, cols...); err != nil {
		return 0, err
	}
	return nextGeneration, nil
}

func validateOpenEndpointID(endpointID string) error {
	if endpointID == workspaceEndpointID || endpointIDPattern.MatchString(endpointID) {
		return nil
	}
	return errors.New("invalid endpoint_id")
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

func openTokenCodeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func denyOpenToken(category string) *codespacev1.ValidateOpenTokenResponse {
	return &codespacev1.ValidateOpenTokenResponse{
		Outcome: &codespacev1.ValidateOpenTokenResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: category},
		},
	}
}
