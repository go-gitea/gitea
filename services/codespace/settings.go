// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
)

const (
	// ManagerSettingsScopeSite selects the site-wide Codespace settings view.
	ManagerSettingsScopeSite = "site"
	// ManagerSettingsScopeUser selects one user's Codespace settings view.
	ManagerSettingsScopeUser = "user"
)

var (
	// ErrManagerSettingsNotFound is returned when a Manager is outside the requested scope.
	ErrManagerSettingsNotFound = errors.New("codespace manager settings target not found")
	// ErrManagerSettingsConfirmRequired is returned when a destructive settings action lacks confirmation.
	ErrManagerSettingsConfirmRequired = errors.New("codespace manager settings confirmation required")
	// ErrManagerSettingsOwnershipConflict is returned before personal deletion when a binding crosses the owner scope.
	ErrManagerSettingsOwnershipConflict = errors.New("codespace manager contains a Codespace outside the owner scope")
	// ErrManagerSettingsNameInvalid is returned when a Manager display name cannot be stored.
	ErrManagerSettingsNameInvalid = errors.New("codespace manager name is invalid")
)

// ManagerSettingsOptions selects site-wide or personal Codespace settings.
type ManagerSettingsOptions struct {
	Scope  string
	UserID int64
}

// DeleteManagerOptions identifies one Manager deletion request.
type DeleteManagerOptions struct {
	Scope     string
	UserID    int64
	ManagerID int64
	Confirm   bool
}

// CreateManagerOptions contains the settings scope and Gitea-managed display name.
type CreateManagerOptions struct {
	ManagerSettingsOptions
	Name string
}

// CreateManagerResult returns the Manager identity and one-time plaintext secret.
type CreateManagerResult struct {
	ManagerID int64
	Name      string
	Secret    string
}

// ManagerDetailOptions selects one Manager management page and its Codespace page.
type ManagerDetailOptions struct {
	ManagerSettingsOptions
	ManagerID int64
	Page      int
	PageSize  int
}

// ManagerDetail contains one Manager and its scoped Codespace governance rows.
type ManagerDetail struct {
	Manager    *ManagerSettingsView
	Codespaces []*GovernanceView
	Total      int64
}

// ManagerSettings contains Manager rows for settings pages.
type ManagerSettings struct {
	Managers []*ManagerSettingsView
}

// ManagerSettingsView contains fields shown on Manager settings pages.
type ManagerSettingsView struct {
	ID                                 int64
	Name                               string
	UserID                             int64
	UserDisplayName                    string
	Version                            string
	RuntimeDisplayState                string
	Environments                       []ManagerEnvironmentDeclaration
	EnvironmentDescriptionConflicts    []string
	LastOnlineUnix                     int64
	CreatedUnix                        int64
	GatewayURL                         string
	GatewaySSHAddr                     string
	GatewaySSHHostKeyAlgorithm         string
	GatewaySSHHostKeyFingerprintSHA256 string
	GatewaySSHHostKeyUpdatedUnix       int64
	BoundCodespaces                    int64
}

// ListManagerSettings returns Manager summaries for one settings page.
func ListManagerSettings(ctx context.Context, opts ManagerSettingsOptions) (*ManagerSettings, error) {
	if err := validateManagerSettingsScope(ctx, opts); err != nil {
		return nil, err
	}
	var managers []*codespace_model.Manager
	query := db.GetEngine(ctx)
	if opts.Scope != ManagerSettingsScopeSite {
		query = query.Where("user_id = ?", opts.UserID)
	}
	if err := query.Asc("user_id", "id").Find(&managers); err != nil {
		return nil, err
	}
	views, err := settingsManagerViews(ctx, managers, opts.UserID)
	if err != nil {
		return nil, err
	}
	return &ManagerSettings{Managers: views}, nil
}

// GetManagerDetail returns one Manager only when it belongs to the requested settings scope.
func GetManagerDetail(ctx context.Context, opts ManagerDetailOptions) (*ManagerDetail, error) {
	if err := validateManagerSettingsScope(ctx, opts.ManagerSettingsOptions); err != nil {
		return nil, err
	}
	if opts.ManagerID <= 0 || opts.Page <= 0 || opts.PageSize <= 0 {
		return nil, ErrManagerSettingsNotFound
	}
	manager, err := loadSettingsManager(ctx, opts.ManagerID)
	if err != nil {
		return nil, err
	}
	if manager == nil || !managerInSettingsScope(manager, opts.Scope, opts.UserID) {
		return nil, ErrManagerSettingsNotFound
	}
	views, err := settingsManagerViews(ctx, []*codespace_model.Manager{manager}, opts.UserID)
	if err != nil {
		return nil, err
	}
	views[0].EnvironmentDescriptionConflicts, err = findEnvironmentDescriptionConflicts(ctx, views[0].Environments, opts.ManagerSettingsOptions)
	if err != nil {
		return nil, err
	}
	list, err := ListGovernanceCodespaces(ctx, GovernanceListOptions{
		ManagerID: manager.ID,
		UserID:    opts.UserID,
		Page:      opts.Page,
		PageSize:  opts.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return &ManagerDetail{Manager: views[0], Codespaces: list.Rows, Total: list.Total}, nil
}

// CreateManager creates a Manager identity and returns its secret once.
func CreateManager(ctx context.Context, opts CreateManagerOptions) (*CreateManagerResult, error) {
	name, err := normalizeManagerDisplayName(opts.Name)
	if err != nil {
		return nil, err
	}
	if err := validateManagerSettingsScope(ctx, opts.ManagerSettingsOptions); err != nil {
		return nil, err
	}
	userID := opts.UserID
	if opts.Scope == ManagerSettingsScopeSite {
		userID = 0
	}
	result := new(CreateManagerResult)
	err = globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if err := validateManagerSettingsScope(ctx, opts.ManagerSettingsOptions); err != nil {
				return err
			}
			manager := &codespace_model.Manager{
				Name:         name,
				UserID:       userID,
				RuntimeState: codespace_model.ManagerRuntimeStateRecovering,
				TagsJSON:     "[]",
				CreatedUnix:  time.Now().Unix(),
			}
			result.Secret = manager.GenerateManagerSecret()
			if _, err := db.GetEngine(ctx).Insert(manager); err != nil {
				return err
			}
			result.ManagerID = manager.ID
			result.Name = manager.Name
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeManagerDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 {
		return "", ErrManagerSettingsNameInvalid
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrManagerSettingsNameInvalid
		}
	}
	return name, nil
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
	if manager == nil || !managerInSettingsScope(manager, opts.Scope, opts.UserID) {
		return ErrManagerSettingsNotFound
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(manager.UserID), func(ctx context.Context) error {
		return deleteManagerIdentityLocked(ctx, manager.ID, 100, func(current *codespace_model.Manager) (bool, error) {
			if !managerInSettingsScope(current, opts.Scope, opts.UserID) {
				return false, ErrManagerSettingsNotFound
			}
			if opts.Scope == ManagerSettingsScopeUser {
				foreignBinding, err := db.GetEngine(ctx).
					Where("manager_id = ? AND user_id <> ?", current.ID, opts.UserID).
					Exist(new(codespace_model.Codespace))
				if err != nil {
					return false, err
				}
				if foreignBinding {
					return false, ErrManagerSettingsOwnershipConflict
				}
			}
			return true, nil
		})
	})
}

func deleteManagerIdentityLocked(ctx context.Context, managerID int64, batchSize int, validate func(*codespace_model.Manager) (bool, error)) error {
	return globallock.LockAndDo(ctx, fetchManagerLockKey(managerID), func(ctx context.Context) error {
		if validate != nil {
			current, err := loadSettingsManager(ctx, managerID)
			if err != nil {
				return err
			}
			if current == nil {
				return nil
			}
			ok, err := validate(current)
			if err != nil || !ok {
				return err
			}
		}
		for {
			var rows []*codespace_model.Codespace
			if err := db.GetEngine(ctx).
				Where("manager_id = ?", managerID).
				Asc("id").
				Limit(batchSize).
				Find(&rows); err != nil {
				return err
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				if err := deleteManagerCodespace(ctx, managerID, row.UUID); err != nil {
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

func settingsManagerViews(ctx context.Context, managers []*codespace_model.Manager, scopeUserID int64) ([]*ManagerSettingsView, error) {
	result := make([]*ManagerSettingsView, 0, len(managers))
	if len(managers) == 0 {
		return result, nil
	}

	managerIDs := make([]int64, 0, len(managers))
	userIDs := make([]int64, 0, len(managers))
	seenUserIDs := make(map[int64]struct{}, len(managers))
	for _, manager := range managers {
		managerIDs = append(managerIDs, manager.ID)
		if manager.UserID > 0 {
			if _, seen := seenUserIDs[manager.UserID]; !seen {
				seenUserIDs[manager.UserID] = struct{}{}
				userIDs = append(userIDs, manager.UserID)
			}
		}
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	type managerAddresses struct {
		gatewayURL     string
		gatewaySSHAddr string
	}
	addressesByManager := make(map[int64]managerAddresses, len(managers))
	boundCodespacesByManager := make(map[int64]int64, len(managers))
	for start := 0; start < len(managerIDs); start += db.DefaultMaxInSize {
		end := min(start+db.DefaultMaxInSize, len(managerIDs))
		ids := managerIDs[start:end]

		var addresses []*codespace_model.ManagerAddress
		if err := db.GetEngine(ctx).In("manager_id", ids).Asc("manager_id", "kind").Find(&addresses); err != nil {
			return nil, err
		}
		for _, address := range addresses {
			values := addressesByManager[address.ManagerID]
			switch address.Kind {
			case codespace_model.ManagerAddressGateway:
				values.gatewayURL = address.Address
			case codespace_model.ManagerAddressSSH:
				values.gatewaySSHAddr = address.Address
			}
			addressesByManager[address.ManagerID] = values
		}

		counts := make([]*struct {
			ManagerID int64
			Count     int64
		}, 0, len(ids))
		query := db.GetEngine(ctx).In("manager_id", ids)
		if scopeUserID > 0 {
			query = query.Where("user_id = ?", scopeUserID)
		}
		if err := query.
			Select("manager_id AS manager_id, COUNT(*) AS count").
			Table("codespace").
			GroupBy("manager_id").
			Find(&counts); err != nil {
			return nil, err
		}
		for _, count := range counts {
			boundCodespacesByManager[count.ManagerID] = count.Count
		}
	}

	for _, manager := range managers {
		environments, err := decodeManagerEnvironments(manager)
		if err != nil {
			return nil, err
		}
		userName := "Global"
		if manager.UserID > 0 {
			if user := users[manager.UserID]; user != nil {
				userName = user.DisplayName()
			} else {
				userName = fmt.Sprintf("User %d", manager.UserID)
			}
		}
		runtimeState := manager.RuntimeState
		if runtimeState == "" {
			runtimeState = codespace_model.ManagerRuntimeStateRecovering
		}
		addresses := addressesByManager[manager.ID]
		view := &ManagerSettingsView{
			ID:                                 manager.ID,
			Name:                               manager.Name,
			UserID:                             manager.UserID,
			UserDisplayName:                    userName,
			Version:                            manager.Version,
			RuntimeDisplayState:                runtimeState,
			Environments:                       environments,
			LastOnlineUnix:                     manager.LastOnlineUnix,
			CreatedUnix:                        manager.CreatedUnix,
			GatewayURL:                         addresses.gatewayURL,
			GatewaySSHAddr:                     addresses.gatewaySSHAddr,
			GatewaySSHHostKeyAlgorithm:         manager.GatewaySSHHostKeyAlgorithm,
			GatewaySSHHostKeyFingerprintSHA256: manager.GatewaySSHHostKeyFingerprintSHA256,
			GatewaySSHHostKeyUpdatedUnix:       manager.GatewaySSHHostKeyUpdatedUnix,
			BoundCodespaces:                    boundCodespacesByManager[manager.ID],
		}
		if isManagerOffline(manager) {
			view.RuntimeDisplayState = managerDisplayOffline
		}
		if view.Name == "" {
			view.Name = fmt.Sprintf("Manager %d", manager.ID)
		}
		result = append(result, view)
	}
	return result, nil
}

func findEnvironmentDescriptionConflicts(ctx context.Context, targetEnvironments []ManagerEnvironmentDeclaration, opts ManagerSettingsOptions) ([]string, error) {
	var managers []*codespace_model.Manager
	query := db.GetEngine(ctx).Where("last_online_unix > 0")
	if opts.Scope == ManagerSettingsScopeUser {
		query = query.In("user_id", []int64{0, opts.UserID})
	}
	if err := query.Find(&managers); err != nil {
		return nil, err
	}

	descriptions := make(map[string]map[string]struct{})
	for _, manager := range managers {
		environments, err := decodeManagerEnvironments(manager)
		if err != nil {
			return nil, err
		}
		for _, environment := range environments {
			if environment.Description == "" {
				continue
			}
			if descriptions[environment.Tag] == nil {
				descriptions[environment.Tag] = make(map[string]struct{})
			}
			descriptions[environment.Tag][environment.Description] = struct{}{}
		}
	}

	conflicts := make([]string, 0)
	for _, environment := range targetEnvironments {
		if len(descriptions[environment.Tag]) > 1 {
			conflicts = append(conflicts, environment.Tag)
		}
	}
	return conflicts, nil
}

func validateManagerSettingsScope(ctx context.Context, opts ManagerSettingsOptions) error {
	switch opts.Scope {
	case ManagerSettingsScopeSite:
		if opts.UserID != 0 {
			return errors.New("site settings user_id must be 0")
		}
		return nil
	case ManagerSettingsScopeUser:
		if opts.UserID <= 0 {
			return errors.New("user_id must be positive")
		}
		user, err := user_model.GetUserByID(ctx, opts.UserID)
		if err != nil {
			return err
		}
		if user.Type != user_model.UserTypeIndividual {
			return errors.New("user is not an individual")
		}
		return nil
	default:
		return fmt.Errorf("unsupported manager settings scope %q", opts.Scope)
	}
}

func loadSettingsManager(ctx context.Context, managerID int64) (*codespace_model.Manager, error) {
	manager := new(codespace_model.Manager)
	has, err := db.GetEngine(ctx).ID(managerID).Get(manager)
	if err != nil || !has {
		return nil, err
	}
	return manager, nil
}

func managerInSettingsScope(manager *codespace_model.Manager, scope string, userID int64) bool {
	if manager == nil {
		return false
	}
	switch scope {
	case ManagerSettingsScopeSite:
		return true
	case ManagerSettingsScopeUser:
		return manager.UserID == userID
	default:
		return false
	}
}

func deleteManagerCodespace(ctx context.Context, managerID int64, codespaceUUID string) error {
	return globallock.LockAndDo(ctx, codespaceStateLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).Where("uuid = ?", codespaceUUID).Get(codespace)
			if err != nil || !has || codespace.ManagerID != managerID {
				return err
			}
			return deleteCodespaceForFinal(ctx, codespaceUUID)
		})
	})
}
