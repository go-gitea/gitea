// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

var (
	environmentTagPattern       = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	sshHostKeyFingerprintRegexp = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]+={0,2}$`)
)

const managerMaxEnvironments = 64

var (
	// ErrManagerUnregistered is returned when the Manager ID has no current row.
	ErrManagerUnregistered = errors.New("manager unregistered")
	// ErrManagerUnauthenticated is returned when the Manager credential is not valid.
	ErrManagerUnauthenticated = errors.New("manager unauthenticated")
	// ErrManagerRegistrationTokenInvalid is returned when a token cannot register a Manager.
	ErrManagerRegistrationTokenInvalid = errors.New("manager registration token is invalid")
)

// DeclareManagerOptions contains the full Manager declaration accepted by Gitea.
type DeclareManagerOptions struct {
	GatewayURL                         string
	GatewaySSHAddr                     string
	Environments                       []*codespacev1.EnvironmentTag
	Version                            string
	RuntimeState                       codespacev1.ManagerRuntimeState
	GatewaySSHHostKeyAlgorithm         string
	GatewaySSHHostKeyFingerprintSHA256 string
}

// ManagerEnvironmentDeclaration is one environment in a Manager declaration snapshot.
type ManagerEnvironmentDeclaration struct {
	Tag         string `json:"tag"`
	Description string `json:"description,omitempty"`
}

// RegisterManager creates a complete Manager identity in the registration token's scope.
func RegisterManager(ctx context.Context, token, name string) (*codespace_model.Manager, string, error) {
	name, err := normalizeManagerDisplayName(name)
	if err != nil {
		return nil, "", err
	}
	if len(token) != 40 {
		return nil, "", ErrManagerRegistrationTokenInvalid
	}
	registrationToken, err := codespace_model.GetManagerRegistrationToken(ctx, token)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, "", ErrManagerRegistrationTokenInvalid
		}
		return nil, "", err
	}
	if !registrationToken.IsActive {
		return nil, "", ErrManagerRegistrationTokenInvalid
	}

	manager := &codespace_model.Manager{}
	var secret string
	err = globallock.LockAndDo(ctx, codespaceUserRelationLockKey(registrationToken.UserID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentToken, err := codespace_model.GetManagerRegistrationToken(ctx, token)
			if err != nil {
				if errors.Is(err, util.ErrNotExist) {
					return ErrManagerRegistrationTokenInvalid
				}
				return err
			}
			if !currentToken.IsActive || currentToken.UserID != registrationToken.UserID {
				return ErrManagerRegistrationTokenInvalid
			}
			if currentToken.UserID > 0 {
				user, err := user_model.GetUserByID(ctx, currentToken.UserID)
				if err != nil || user.Type != user_model.UserTypeIndividual {
					return ErrManagerRegistrationTokenInvalid
				}
			}
			manager = &codespace_model.Manager{
				Name:         name,
				UserID:       currentToken.UserID,
				TagsJSON:     "[]",
				RuntimeState: codespace_model.ManagerRuntimeStateRecovering,
				CreatedUnix:  time.Now().Unix(),
			}
			secret = manager.GenerateManagerSecret()
			_, err = db.GetEngine(ctx).Insert(manager)
			return err
		})
	})
	if err != nil {
		return nil, "", err
	}
	return manager, secret, nil
}

// AuthenticateManager verifies a Manager id and plaintext secret.
func AuthenticateManager(ctx context.Context, managerID int64, secret string) (*codespace_model.Manager, error) {
	if managerID <= 0 || strings.TrimSpace(secret) == "" {
		return nil, ErrManagerUnauthenticated
	}
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrManagerUnregistered
	}
	if !manager.VerifyManagerSecret(secret) {
		return nil, ErrManagerUnauthenticated
	}
	return manager, nil
}

// DeclareManager stores the latest Manager declaration and replaces routable addresses atomically.
func DeclareManager(ctx context.Context, manager *codespace_model.Manager, opts DeclareManagerOptions) error {
	if manager == nil || manager.ID <= 0 {
		return errors.New("manager is required")
	}
	normalizedOpts, err := normalizeDeclareManagerOptions(opts)
	if err != nil {
		return util.NewInvalidArgumentErrorf("%v", err)
	}
	opts = normalizedOpts

	environments, err := normalizeManagerEnvironments(opts.Environments)
	if err != nil {
		return util.NewInvalidArgumentErrorf("%v", err)
	}
	tagsJSON, err := json.Marshal(environments)
	if err != nil {
		return fmt.Errorf("encode tags: %w", err)
	}
	warnGatewayCookieScopeConflict(manager.ID, opts.GatewayURL)

	return globallock.LockAndDo(ctx, fetchManagerLockKey(manager.ID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentManager := new(codespace_model.Manager)
			has, err := db.GetEngine(ctx).ID(manager.ID).Get(currentManager)
			if err != nil {
				return err
			}
			if !has {
				return ErrManagerUnregistered
			}
			now := time.Now().Unix()
			updates := &codespace_model.Manager{
				TagsJSON:                           string(tagsJSON),
				RuntimeState:                       managerRuntimeStateName(opts.RuntimeState),
				LastOnlineUnix:                     now,
				Version:                            opts.Version,
				GatewayURL:                         opts.GatewayURL,
				GatewaySSHAddr:                     opts.GatewaySSHAddr,
				GatewaySSHHostKeyAlgorithm:         opts.GatewaySSHHostKeyAlgorithm,
				GatewaySSHHostKeyFingerprintSHA256: opts.GatewaySSHHostKeyFingerprintSHA256,
			}
			affected, err := db.GetEngine(ctx).ID(currentManager.ID).Cols(
				"tags_json", "runtime_state", "last_online_unix", "version",
				"gateway_url", "gateway_ssh_addr", "gateway_ssh_host_key_algorithm", "gateway_ssh_host_key_fingerprint_sha256",
			).Update(updates)
			if err != nil {
				return err
			}
			if affected == 0 {
				return ErrManagerUnregistered
			}
			return nil
		})
	})
}

func normalizeDeclareManagerOptions(opts DeclareManagerOptions) (DeclareManagerOptions, error) {
	opts.Version = strings.TrimSpace(opts.Version)
	if opts.Version == "" {
		return opts, errors.New("manager version is required")
	}
	if len(opts.Version) > 64 {
		return opts, errors.New("manager version is too long")
	}
	if opts.RuntimeState != codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE && opts.RuntimeState != codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_RECOVERING {
		return opts, fmt.Errorf("invalid manager runtime state %d", opts.RuntimeState)
	}
	gatewayURL, err := normalizeGatewayURL(opts.GatewayURL)
	if err != nil {
		return opts, err
	}
	opts.GatewayURL = gatewayURL
	gatewaySSHAddr, err := normalizeGatewaySSHAddr(opts.GatewaySSHAddr)
	if err != nil {
		return opts, err
	}
	opts.GatewaySSHAddr = gatewaySSHAddr
	opts.GatewaySSHHostKeyAlgorithm = strings.TrimSpace(opts.GatewaySSHHostKeyAlgorithm)
	if opts.GatewaySSHHostKeyAlgorithm == "" {
		return opts, errors.New("gateway ssh host key algorithm is required")
	}
	if len(opts.GatewaySSHHostKeyAlgorithm) > 64 {
		return opts, errors.New("gateway ssh host key algorithm is too long")
	}
	opts.GatewaySSHHostKeyFingerprintSHA256 = strings.TrimSpace(opts.GatewaySSHHostKeyFingerprintSHA256)
	if !sshHostKeyFingerprintRegexp.MatchString(opts.GatewaySSHHostKeyFingerprintSHA256) {
		return opts, errors.New("invalid gateway ssh host key fingerprint")
	}
	return opts, nil
}

func managerRuntimeStateName(state codespacev1.ManagerRuntimeState) string {
	switch state {
	case codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE:
		return codespace_model.ManagerRuntimeStateOnline
	case codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_RECOVERING:
		return codespace_model.ManagerRuntimeStateRecovering
	default:
		return ""
	}
}

func normalizeManagerEnvironments(environments []*codespacev1.EnvironmentTag) ([]ManagerEnvironmentDeclaration, error) {
	if len(environments) == 0 {
		return nil, errors.New("manager environments are required")
	}
	if len(environments) > managerMaxEnvironments {
		return nil, errors.New("manager environments exceed 64")
	}
	normalized := make([]ManagerEnvironmentDeclaration, 0, len(environments))
	seen := make(map[string]struct{}, len(environments))
	for _, environment := range environments {
		if environment == nil {
			return nil, errors.New("manager environment is required")
		}
		tag := environment.GetTag()
		if !environmentTagPattern.MatchString(tag) {
			return nil, fmt.Errorf("invalid manager environment tag %q", tag)
		}
		if _, exists := seen[tag]; exists {
			return nil, fmt.Errorf("manager environment tag %q is duplicated", tag)
		}
		seen[tag] = struct{}{}
		description := strings.TrimSpace(environment.GetDescription())
		if len(description) > 255 {
			return nil, fmt.Errorf("manager environment %q description is too long", tag)
		}
		normalized = append(normalized, ManagerEnvironmentDeclaration{Tag: tag, Description: description})
	}
	return normalized, nil
}
