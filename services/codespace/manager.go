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
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
)

var (
	tagPattern                  = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	sshHostKeyFingerprintRegexp = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]+={0,2}$`)
)

const managerMaxEnvironments = 64

var (
	// ErrManagerUnregistered is returned when the Manager ID has no current row.
	ErrManagerUnregistered = errors.New("manager unregistered")
	// ErrManagerUnauthenticated is returned when the Manager credential is not valid.
	ErrManagerUnauthenticated = errors.New("manager unauthenticated")
	// ErrBindRuntimeIdentityNotFound is returned when the create operation cannot be found.
	ErrBindRuntimeIdentityNotFound = errors.New("runtime identity target not found")
	// ErrBindRuntimeIdentityStateConflict is returned when the create operation cannot accept a runtime UUID.
	ErrBindRuntimeIdentityStateConflict = errors.New("runtime identity state conflict")
	// ErrBindRuntimeIdentityConflict is returned when the runtime UUID is already bound elsewhere.
	ErrBindRuntimeIdentityConflict = errors.New("runtime identity conflict")
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

// BindRuntimeIdentityOptions contains the manager-created runtime identity for one create operation.
type BindRuntimeIdentityOptions struct {
	CodespaceID       int64
	OperationRVersion int64
	RuntimeUUID       string
}

// ManagerEnvironmentDeclaration is one environment in a Manager declaration snapshot.
type ManagerEnvironmentDeclaration struct {
	Tag         string `json:"tag"`
	Description string `json:"description,omitempty"`
}

// BindRuntimeIdentity stores the runtime UUID allocated by the Manager for an active create.
func BindRuntimeIdentity(ctx context.Context, manager *codespace_model.Manager, opts BindRuntimeIdentityOptions) (string, error) {
	if manager == nil || manager.ID <= 0 || opts.CodespaceID <= 0 || opts.OperationRVersion <= 0 {
		return "", ErrBindRuntimeIdentityNotFound
	}
	if err := codespace_model.ValidateUUID(opts.RuntimeUUID); err != nil {
		return "", err
	}
	var updateErr error
	err := db.WithTx(ctx, func(ctx context.Context) error {
		codespace := new(codespace_model.Codespace)
		has, err := db.GetEngine(ctx).ID(opts.CodespaceID).Get(codespace)
		if err != nil {
			return err
		}
		if !has || codespace.ManagerID != manager.ID || codespace.OperationRVersion != opts.OperationRVersion ||
			codespace_model.ActiveOperationType(codespace) != codespace_model.OperationCreate || !codespace_model.IsOperationRunning(codespace) {
			return ErrBindRuntimeIdentityNotFound
		}
		if codespace.UUID == opts.RuntimeUUID {
			return nil
		}
		if codespace.UUID != "" {
			return ErrBindRuntimeIdentityStateConflict
		}
		used, err := db.GetEngine(ctx).Where("uuid = ?", opts.RuntimeUUID).Exist(new(codespace_model.Codespace))
		if err != nil {
			return err
		}
		if used {
			return ErrBindRuntimeIdentityConflict
		}
		affected, err := db.GetEngine(ctx).Where(
			"id = ? AND uuid IS NULL AND manager_id = ? AND status = ? AND operation_r_version = ? AND operation_started_unix > 0",
			codespace.ID, manager.ID, codespace_model.StatusCreating, opts.OperationRVersion,
		).Cols("uuid", "updated_unix").Update(&codespace_model.Codespace{
			UUID:        opts.RuntimeUUID,
			UpdatedUnix: time.Now().Unix(),
		})
		if err != nil {
			updateErr = err
			return err
		}
		if affected == 0 {
			return ErrBindRuntimeIdentityStateConflict
		}
		return nil
	})
	if updateErr != nil {
		// A unique-constraint error aborts the transaction on PostgreSQL, so
		// identify the conflicting runtime only after WithTx has rolled it back.
		used, queryErr := db.GetEngine(ctx).Where("uuid = ?", opts.RuntimeUUID).Exist(new(codespace_model.Codespace))
		if queryErr == nil && used {
			return "", ErrBindRuntimeIdentityConflict
		}
	}
	return opts.RuntimeUUID, err
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
