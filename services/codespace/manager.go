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
	"gitea.dev/modules/setting"
)

var (
	tagPattern                  = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	sshHostKeyFingerprintRegexp = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]+={0,2}$`)
)

const managerMaxEnvironments = 64

var (
	// ErrRegistrationUnauthenticated is returned when a registration token is not current.
	ErrRegistrationUnauthenticated = errors.New("manager registration unauthenticated")
	// ErrRegistrationStateUnavailable is returned when Codespace is not accepting new Manager registrations.
	ErrRegistrationStateUnavailable = errors.New("manager registration state unavailable")
	// ErrManagerUnregistered is returned when the Manager ID has no current row.
	ErrManagerUnregistered = errors.New("manager unregistered")
	// ErrManagerUnauthenticated is returned when the Manager credential is not valid.
	ErrManagerUnauthenticated = errors.New("manager unauthenticated")
	// ErrDeclareGatewayURLConflict is returned when another Manager already uses the Gateway URL.
	ErrDeclareGatewayURLConflict = errors.New("manager gateway url conflict")
	// ErrDeclareGatewaySSHAddrConflict is returned when another Manager already uses the Gateway SSH address.
	ErrDeclareGatewaySSHAddrConflict = errors.New("manager gateway ssh address conflict")
)

// DeclareManagerOptions contains the full Manager declaration accepted by Gitea.
type DeclareManagerOptions struct {
	GatewayURL                         string
	GatewaySSHAddr                     string
	Environments                       []*codespacev1.EnvironmentTag
	Version                            string
	Name                               string
	RuntimeState                       codespacev1.ManagerRuntimeState
	GatewaySSHHostKeyAlgorithm         string
	GatewaySSHHostKeyFingerprintSHA256 string
	GatewaySSHHostKeyUpdatedUnix       int64
}

// ManagerEnvironmentDeclaration is one environment in a Manager declaration snapshot.
type ManagerEnvironmentDeclaration struct {
	Tag         string `json:"tag"`
	Description string `json:"description,omitempty"`
}

// RegisterManager exchanges the current site or personal registration token for a Manager identity.
func RegisterManager(ctx context.Context, registrationToken string) (*codespace_model.Manager, string, error) {
	if !setting.Codespace.Enabled {
		return nil, "", ErrRegistrationStateUnavailable
	}
	registrationToken = strings.TrimSpace(registrationToken)
	if registrationToken == "" {
		return nil, "", ErrRegistrationUnauthenticated
	}

	token, err := loadRegistrationTokenByValue(ctx, registrationToken)
	if err != nil {
		return nil, "", err
	}
	if token == nil {
		return nil, "", ErrRegistrationUnauthenticated
	}

	var manager *codespace_model.Manager
	var secret string
	err = globallock.LockAndDo(ctx, codespaceUserRelationLockKey(token.UserID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentToken, err := loadRegistrationTokenByValue(ctx, registrationToken)
			if err != nil {
				return err
			}
			if currentToken == nil || currentToken.UserID != token.UserID {
				return ErrRegistrationUnauthenticated
			}
			if currentToken.UserID > 0 {
				user, err := user_model.GetUserByID(ctx, currentToken.UserID)
				if err != nil {
					if user_model.IsErrUserNotExist(err) {
						return ErrRegistrationUnauthenticated
					}
					return err
				}
				if user.Type != user_model.UserTypeIndividual {
					return ErrRegistrationUnauthenticated
				}
			}
			manager = &codespace_model.Manager{
				UserID:       currentToken.UserID,
				RuntimeState: codespace_model.ManagerRuntimeStateRecovering,
				TagsJSON:     "[]",
				CreatedUnix:  time.Now().Unix(),
			}
			secret = manager.GenerateManagerSecret()
			if _, err := db.GetEngine(ctx).Insert(manager); err != nil {
				return err
			}
			return nil
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
		return err
	}
	opts = normalizedOpts

	environments, err := normalizeManagerEnvironments(opts.Environments)
	if err != nil {
		return err
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
			if err := checkManagerAddressConflict(ctx, currentManager.ID, codespace_model.ManagerAddressGateway, opts.GatewayURL); err != nil {
				return err
			}
			if err := checkManagerAddressConflict(ctx, currentManager.ID, codespace_model.ManagerAddressSSH, opts.GatewaySSHAddr); err != nil {
				return err
			}

			now := time.Now().Unix()
			updates := &codespace_model.Manager{
				Name:                               opts.Name,
				TagsJSON:                           string(tagsJSON),
				RuntimeState:                       managerRuntimeStateName(opts.RuntimeState),
				LastOnlineUnix:                     now,
				Version:                            opts.Version,
				GatewaySSHHostKeyAlgorithm:         opts.GatewaySSHHostKeyAlgorithm,
				GatewaySSHHostKeyFingerprintSHA256: opts.GatewaySSHHostKeyFingerprintSHA256,
				GatewaySSHHostKeyUpdatedUnix:       opts.GatewaySSHHostKeyUpdatedUnix,
			}
			affected, err := db.GetEngine(ctx).ID(currentManager.ID).Cols(
				"name", "tags_json", "runtime_state", "last_online_unix", "version",
				"gateway_ssh_host_key_algorithm", "gateway_ssh_host_key_fingerprint_sha256", "gateway_ssh_host_key_updated_unix",
			).Update(updates)
			if err != nil {
				return err
			}
			if affected == 0 {
				return ErrManagerUnregistered
			}
			if _, err := db.GetEngine(ctx).Where("manager_id = ?", currentManager.ID).Delete(new(codespace_model.ManagerAddress)); err != nil {
				return err
			}
			addresses := []*codespace_model.ManagerAddress{
				{ManagerID: currentManager.ID, Kind: codespace_model.ManagerAddressGateway, Address: opts.GatewayURL},
				{ManagerID: currentManager.ID, Kind: codespace_model.ManagerAddressSSH, Address: opts.GatewaySSHAddr},
			}
			if _, err := db.GetEngine(ctx).Insert(addresses); err != nil {
				return err
			}
			return nil
		})
	})
}

func checkManagerAddressConflict(ctx context.Context, managerID int64, kind, address string) error {
	existing := new(codespace_model.ManagerAddress)
	has, err := db.GetEngine(ctx).
		Where("kind = ? AND address = ? AND manager_id <> ?", kind, address, managerID).
		Get(existing)
	if err != nil || !has {
		return err
	}
	switch kind {
	case codespace_model.ManagerAddressGateway:
		return ErrDeclareGatewayURLConflict
	case codespace_model.ManagerAddressSSH:
		return ErrDeclareGatewaySSHAddrConflict
	default:
		return errors.New("manager address conflict")
	}
}

func normalizeDeclareManagerOptions(opts DeclareManagerOptions) (DeclareManagerOptions, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if opts.Name == "" {
		return opts, errors.New("manager name is required")
	}
	if len(opts.Name) > 255 {
		return opts, errors.New("manager name is too long")
	}
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
	if opts.GatewaySSHHostKeyUpdatedUnix < 0 {
		return opts, errors.New("gateway ssh host key updated time must not be negative")
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
		tag := strings.ToLower(strings.TrimSpace(environment.GetTag()))
		if !tagPattern.MatchString(tag) {
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

func loadRegistrationTokenByValue(ctx context.Context, tokenValue string) (*codespace_model.ManagerToken, error) {
	token := new(codespace_model.ManagerToken)
	has, err := db.GetEngine(ctx).Where("token = ?", tokenValue).Get(token)
	if err != nil || !has {
		return nil, err
	}
	return token, nil
}
