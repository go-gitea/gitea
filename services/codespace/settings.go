// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

const (
	// ManagerSettingsScopeSite selects the site-wide Codespace settings view.
	ManagerSettingsScopeSite = "site"
	// ManagerSettingsScopeUser selects one user's Codespace settings view.
	ManagerSettingsScopeUser = "user"
	// ManagerSettingsScopeOrganization selects one organization's Codespace settings view.
	ManagerSettingsScopeOrganization = "organization"
)

var (
	// ErrManagerSettingsNotFound is returned when a Manager is outside the requested scope.
	ErrManagerSettingsNotFound = errors.New("codespace manager settings target not found")
	// ErrManagerSettingsConfirmRequired is returned when a destructive settings action lacks confirmation.
	ErrManagerSettingsConfirmRequired = errors.New("codespace manager settings confirmation required")
)

// ManagerSettingsOptions selects one owner scope for Codespace settings.
type ManagerSettingsOptions struct {
	Scope   string
	OwnerID int64
}

// DeleteManagerOptions identifies one Manager deletion request.
type DeleteManagerOptions struct {
	Scope     string
	OwnerID   int64
	ManagerID int64
	Confirm   bool
}

// ManagerSettings contains registration token and Manager rows for settings pages.
type ManagerSettings struct {
	RegistrationToken string
	Managers          []*ManagerSettingsView
}

// ManagerSettingsView contains fields shown on owner-scoped settings pages.
type ManagerSettingsView struct {
	ID                                 int64
	Name                               string
	OwnerID                            int64
	OwnerDisplayName                   string
	Version                            string
	RuntimeState                       string
	RuntimeDisplayState                string
	Tags                               []string
	LastOnlineUnix                     int64
	CreatedUnix                        int64
	GatewayURL                         string
	GatewaySSHAddr                     string
	GatewaySSHHostKeyAlgorithm         string
	GatewaySSHHostKeyFingerprintSHA256 string
	GatewaySSHHostKeyUpdatedUnix       int64
	BoundCodespaces                    int64
	CanDelete                          bool
}

type managerSettingsMetadata struct {
	Version                            string `json:"version"`
	GatewaySSHHostKeyAlgorithm         string `json:"gateway_ssh_host_key_algorithm"`
	GatewaySSHHostKeyFingerprintSHA256 string `json:"gateway_ssh_host_key_fingerprint_sha256"`
	GatewaySSHHostKeyUpdatedUnix       int64  `json:"gateway_ssh_host_key_updated_unix"`
}

// ListManagerSettings returns the current token row and Manager summaries for one settings page.
func ListManagerSettings(ctx context.Context, opts ManagerSettingsOptions) (*ManagerSettings, error) {
	if err := validateManagerSettingsScope(ctx, opts); err != nil {
		return nil, err
	}
	result := &ManagerSettings{}
	token, err := GetOrCreateRegistrationToken(ctx, opts)
	if err != nil {
		return nil, err
	}
	result.RegistrationToken = token
	managers, err := listSettingsManagers(ctx, opts)
	if err != nil {
		return nil, err
	}
	result.Managers = managers
	return result, nil
}

// GetOrCreateRegistrationToken returns the current token or creates one for the owner scope.
func GetOrCreateRegistrationToken(ctx context.Context, opts ManagerSettingsOptions) (string, error) {
	if err := validateManagerSettingsScope(ctx, opts); err != nil {
		return "", err
	}
	ownerID := registrationTokenOwnerID(opts)
	var tokenValue string
	err := globallock.LockAndDo(ctx, codespaceOwnerRelationLockKey(ownerID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if err := validateManagerSettingsOwnerInTx(ctx, opts); err != nil {
				return err
			}
			token, err := loadRegistrationTokenByOwner(ctx, ownerID)
			if err != nil {
				return err
			}
			if token != nil {
				tokenValue = token.Token
				return nil
			}
			tokenValue = newRegistrationToken()
			_, err = db.GetEngine(ctx).Insert(&codespace_model.ManagerToken{
				OwnerID: ownerID,
				Token:   tokenValue,
			})
			return err
		})
	})
	return tokenValue, err
}

// ResetRegistrationToken replaces the current owner token in place and returns the new value.
func ResetRegistrationToken(ctx context.Context, opts ManagerSettingsOptions) (string, error) {
	if err := validateManagerSettingsScope(ctx, opts); err != nil {
		return "", err
	}
	ownerID := registrationTokenOwnerID(opts)
	var tokenValue string
	err := globallock.LockAndDo(ctx, codespaceOwnerRelationLockKey(ownerID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if err := validateManagerSettingsOwnerInTx(ctx, opts); err != nil {
				return err
			}
			tokenValue = newRegistrationToken()
			token, err := loadRegistrationTokenByOwner(ctx, ownerID)
			if err != nil {
				return err
			}
			if token == nil {
				_, err = db.GetEngine(ctx).Insert(&codespace_model.ManagerToken{
					OwnerID: ownerID,
					Token:   tokenValue,
				})
				return err
			}
			token.Token = tokenValue
			_, err = db.GetEngine(ctx).ID(token.ID).Cols("token").Update(token)
			return err
		})
	})
	return tokenValue, err
}

// DeleteManager removes one Manager identity and all Gitea records bound to it.
func DeleteManager(ctx context.Context, opts DeleteManagerOptions) error {
	if !opts.Confirm {
		return ErrManagerSettingsConfirmRequired
	}
	if opts.ManagerID <= 0 {
		return ErrManagerSettingsNotFound
	}
	manager, err := loadSettingsManager(ctx, opts.ManagerID)
	if err != nil {
		return err
	}
	if manager == nil || !managerInSettingsScope(manager, opts.Scope, opts.OwnerID) {
		return ErrManagerSettingsNotFound
	}
	return globallock.LockAndDo(ctx, codespaceOwnerRelationLockKey(manager.OwnerID), func(ctx context.Context) error {
		return deleteManagerIdentityLocked(ctx, manager.ID, 100, func(current *codespace_model.Manager) (bool, error) {
			if !managerInSettingsScope(current, opts.Scope, opts.OwnerID) {
				return false, ErrManagerSettingsNotFound
			}
			return true, nil
		})
	})
}

func deleteManagerIdentityLocked(ctx context.Context, managerID int64, batchSize int, validate func(*codespace_model.Manager) (bool, error)) error {
	return globallock.LockAndDo(ctx, fetchManagerLockKey(managerID), func(ctx context.Context) error {
		for {
			codespaceUUIDs, err := listManagerCodespaceUUIDs(ctx, managerID, batchSize)
			if err != nil {
				return err
			}
			if len(codespaceUUIDs) == 0 {
				break
			}
			for _, codespaceUUID := range codespaceUUIDs {
				if err := deleteManagerCodespace(ctx, managerID, codespaceUUID); err != nil {
					return err
				}
			}
		}
		return db.WithTx(ctx, func(ctx context.Context) error {
			current, err := loadSettingsManager(ctx, managerID)
			if err != nil {
				return err
			}
			if current == nil {
				return nil
			}
			if validate != nil {
				ok, err := validate(current)
				if err != nil || !ok {
					return err
				}
			}
			hasCodespace, err := db.GetEngine(ctx).Where("manager_id = ?", current.ID).Exist(new(codespace_model.Codespace))
			if err != nil {
				return err
			}
			if hasCodespace {
				return fmt.Errorf("manager %d still has bound codespaces", current.ID)
			}
			if _, err := db.GetEngine(ctx).Where("manager_id = ?", current.ID).Delete(new(codespace_model.ManagerAddress)); err != nil {
				return err
			}
			_, err = db.GetEngine(ctx).ID(current.ID).Delete(new(codespace_model.Manager))
			return err
		})
	})
}

func listSettingsManagers(ctx context.Context, opts ManagerSettingsOptions) ([]*ManagerSettingsView, error) {
	var managers []*codespace_model.Manager
	query := db.GetEngine(ctx)
	if opts.Scope != ManagerSettingsScopeSite {
		query = query.Where("owner_id = ?", opts.OwnerID)
	}
	if err := query.Asc("owner_id", "id").Find(&managers); err != nil {
		return nil, err
	}
	result := make([]*ManagerSettingsView, 0, len(managers))
	owners := make(map[int64]string)
	for _, manager := range managers {
		view, err := settingsManagerView(ctx, manager, owners)
		if err != nil {
			return nil, err
		}
		view.CanDelete = true
		result = append(result, view)
	}
	return result, nil
}

func settingsManagerView(ctx context.Context, manager *codespace_model.Manager, owners map[int64]string) (*ManagerSettingsView, error) {
	tags, err := decodeManagerTags(manager)
	if err != nil {
		return nil, err
	}
	metadata, err := decodeManagerSettingsMetadata(manager.MetaJSON)
	if err != nil {
		return nil, err
	}
	gatewayURL, gatewaySSHAddr, err := settingsManagerAddresses(ctx, manager.ID)
	if err != nil {
		return nil, err
	}
	boundCodespaces, err := db.GetEngine(ctx).Where("manager_id = ?", manager.ID).Count(new(codespace_model.Codespace))
	if err != nil {
		return nil, err
	}
	ownerName, err := settingsManagerOwnerDisplayName(ctx, owners, manager.OwnerID)
	if err != nil {
		return nil, err
	}
	runtimeState := manager.RuntimeState
	if runtimeState == "" {
		runtimeState = codespace_model.ManagerRuntimeStateRecovering
	}
	view := &ManagerSettingsView{
		ID:                                 manager.ID,
		Name:                               manager.Name,
		OwnerID:                            manager.OwnerID,
		OwnerDisplayName:                   ownerName,
		Version:                            metadata.Version,
		RuntimeState:                       runtimeState,
		RuntimeDisplayState:                runtimeState,
		Tags:                               tags,
		LastOnlineUnix:                     manager.LastOnlineUnix,
		CreatedUnix:                        manager.CreatedUnix,
		GatewayURL:                         gatewayURL,
		GatewaySSHAddr:                     gatewaySSHAddr,
		GatewaySSHHostKeyAlgorithm:         metadata.GatewaySSHHostKeyAlgorithm,
		GatewaySSHHostKeyFingerprintSHA256: metadata.GatewaySSHHostKeyFingerprintSHA256,
		GatewaySSHHostKeyUpdatedUnix:       metadata.GatewaySSHHostKeyUpdatedUnix,
		BoundCodespaces:                    boundCodespaces,
	}
	if isManagerOffline(manager) {
		view.RuntimeDisplayState = managerDisplayOffline
	}
	if view.Name == "" {
		view.Name = fmt.Sprintf("Manager %d", manager.ID)
	}
	return view, nil
}

func decodeManagerSettingsMetadata(metaJSON string) (*managerSettingsMetadata, error) {
	metadata := new(managerSettingsMetadata)
	if strings.TrimSpace(metaJSON) == "" {
		return metadata, nil
	}
	if err := json.Unmarshal([]byte(metaJSON), metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

func settingsManagerAddresses(ctx context.Context, managerID int64) (string, string, error) {
	var addresses []*codespace_model.ManagerAddress
	if err := db.GetEngine(ctx).Where("manager_id = ?", managerID).Find(&addresses); err != nil {
		return "", "", err
	}
	var gatewayURL, gatewaySSHAddr string
	for _, address := range addresses {
		switch address.Kind {
		case codespace_model.ManagerAddressGateway:
			gatewayURL = address.Address
		case codespace_model.ManagerAddressSSH:
			gatewaySSHAddr = address.Address
		}
	}
	return gatewayURL, gatewaySSHAddr, nil
}

func settingsManagerOwnerDisplayName(ctx context.Context, cache map[int64]string, ownerID int64) (string, error) {
	if ownerID == 0 {
		return "Global", nil
	}
	if name, ok := cache[ownerID]; ok {
		return name, nil
	}
	owner, err := user_model.GetUserByID(ctx, ownerID)
	if user_model.IsErrUserNotExist(err) {
		cache[ownerID] = fmt.Sprintf("Owner %d", ownerID)
		return cache[ownerID], nil
	}
	if err != nil {
		return "", err
	}
	cache[ownerID] = owner.DisplayName()
	return cache[ownerID], nil
}

func loadRegistrationTokenByOwner(ctx context.Context, ownerID int64) (*codespace_model.ManagerToken, error) {
	token := new(codespace_model.ManagerToken)
	has, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Get(token)
	if err != nil || !has {
		return nil, err
	}
	return token, nil
}

func registrationTokenOwnerID(opts ManagerSettingsOptions) int64 {
	if opts.Scope == ManagerSettingsScopeSite {
		return 0
	}
	return opts.OwnerID
}

func validateManagerSettingsScope(ctx context.Context, opts ManagerSettingsOptions) error {
	switch opts.Scope {
	case ManagerSettingsScopeSite:
		if opts.OwnerID != 0 {
			return errors.New("site settings owner_id must be 0")
		}
		return nil
	case ManagerSettingsScopeUser, ManagerSettingsScopeOrganization:
		if opts.OwnerID <= 0 {
			return errors.New("owner_id must be positive")
		}
		return validateManagerSettingsOwner(ctx, opts)
	default:
		return fmt.Errorf("unsupported manager settings scope %q", opts.Scope)
	}
}

func validateManagerSettingsOwner(ctx context.Context, opts ManagerSettingsOptions) error {
	owner, err := user_model.GetUserByID(ctx, opts.OwnerID)
	if err != nil {
		return err
	}
	return validateManagerSettingsOwnerType(owner, opts.Scope)
}

func validateManagerSettingsOwnerInTx(ctx context.Context, opts ManagerSettingsOptions) error {
	if opts.Scope == ManagerSettingsScopeSite {
		return nil
	}
	owner, err := user_model.GetUserByID(ctx, opts.OwnerID)
	if err != nil {
		return err
	}
	return validateManagerSettingsOwnerType(owner, opts.Scope)
}

func validateManagerSettingsOwnerType(owner *user_model.User, scope string) error {
	if owner == nil {
		return errors.New("owner is required")
	}
	switch scope {
	case ManagerSettingsScopeUser:
		if owner.Type != user_model.UserTypeIndividual {
			return errors.New("owner is not a user")
		}
	case ManagerSettingsScopeOrganization:
		if owner.Type != user_model.UserTypeOrganization {
			return errors.New("owner is not an organization")
		}
	}
	return nil
}

func loadSettingsManager(ctx context.Context, managerID int64) (*codespace_model.Manager, error) {
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil || !has {
		return nil, err
	}
	return manager, nil
}

func managerInSettingsScope(manager *codespace_model.Manager, scope string, ownerID int64) bool {
	if manager == nil {
		return false
	}
	switch scope {
	case ManagerSettingsScopeSite:
		return true
	case ManagerSettingsScopeUser, ManagerSettingsScopeOrganization:
		return manager.OwnerID == ownerID
	default:
		return false
	}
}

func listManagerCodespaceUUIDs(ctx context.Context, managerID int64, limit int) ([]string, error) {
	var rows []*codespace_model.Codespace
	if err := db.GetEngine(ctx).
		Where("manager_id = ?", managerID).
		Asc("uuid").
		Limit(limit).
		Find(&rows); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.UUID)
	}
	return result, nil
}

func deleteManagerCodespace(ctx context.Context, managerID int64, codespaceUUID string) error {
	return globallock.LockAndDo(ctx, codespaceLifecycleActionLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
			if err != nil || !has || codespace.ManagerID != managerID {
				return err
			}
			return deleteCodespaceForFinal(ctx, codespaceUUID)
		})
	})
}

func newRegistrationToken() string {
	return strings.ToLower(hex.EncodeToString(util.CryptoRandomBytes(32)))
}
