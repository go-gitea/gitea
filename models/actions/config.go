// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm/convert"
)

// OwnerActionsConfig defines the Actions configuration for a user or organization
type OwnerActionsConfig struct {
	// TokenPermissionMode defines the default permission mode (permissive, restricted)
	TokenPermissionMode repo_model.ActionsTokenPermissionMode `json:"token_permission_mode,omitempty"`

	// MaxTokenPermissions defines the absolute maximum permissions any token can have in this context.
	MaxTokenPermissions *repo_model.ActionsTokenPermissions `json:"max_token_permissions,omitempty"`

	// AllowedCrossRepoIDs is a list of specific repo IDs that can be accessed cross-repo
	AllowedCrossRepoIDs []int64 `json:"allowed_cross_repo_ids,omitempty"`
}

var _ convert.ConversionFrom = (*OwnerActionsConfig)(nil)

func (cfg *OwnerActionsConfig) FromDB(bytes []byte) error {
	_ = json.Unmarshal(bytes, cfg)
	cfg.TokenPermissionMode, _ = util.EnumValue(cfg.TokenPermissionMode)
	return nil
}

// GetOwnerActionsConfig loads the OwnerActionsConfig for a user or organization from user settings
// It returns a default config if no setting is found
func GetOwnerActionsConfig(ctx context.Context, userID int64) (ret OwnerActionsConfig, err error) {
	return user_model.GetUserSettingJSON(ctx, userID, user_model.SettingsKeyActionsConfig, ret)
}

// SetOwnerActionsConfig saves the OwnerActionsConfig for a user or organization to user settings
func SetOwnerActionsConfig(ctx context.Context, userID int64, cfg OwnerActionsConfig) error {
	return user_model.SetUserSettingJSON(ctx, userID, user_model.SettingsKeyActionsConfig, cfg)
}

// GetDefaultTokenPermissions returns the default token permissions by its TokenPermissionMode.
func (cfg *OwnerActionsConfig) GetDefaultTokenPermissions() repo_model.ActionsTokenPermissions {
	switch cfg.TokenPermissionMode {
	case repo_model.ActionsTokenPermissionModeRestricted:
		return repo_model.MakeRestrictedPermissions()
	case repo_model.ActionsTokenPermissionModePermissive:
		return repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite)
	default:
		return repo_model.MakeActionsTokenPermissions(perm.AccessModeNone)
	}
}

// GetMaxTokenPermissions returns the maximum allowed permissions
func (cfg *OwnerActionsConfig) GetMaxTokenPermissions() repo_model.ActionsTokenPermissions {
	if cfg.MaxTokenPermissions != nil {
		return *cfg.MaxTokenPermissions
	}
	// Default max is write for everything
	return repo_model.MakeActionsTokenPermissions(perm.AccessModeWrite)
}

// ClampPermissions ensures that the given permissions don't exceed the maximum
func (cfg *OwnerActionsConfig) ClampPermissions(perms repo_model.ActionsTokenPermissions) repo_model.ActionsTokenPermissions {
	maxPerms := cfg.GetMaxTokenPermissions()
	return repo_model.ClampActionsTokenPermissions(perms, maxPerms)
}
