// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// UserActionsConfig defines the Actions configuration for a user or organization
type UserActionsConfig struct {
	// TokenPermissionMode defines the default permission mode (permissive, restricted, or custom)
	TokenPermissionMode repo_model.ActionsTokenPermissionMode `json:"token_permission_mode,omitempty"`
	// MaxTokenPermissions defines the absolute maximum permissions any token can have in this context.
	MaxTokenPermissions *repo_model.ActionsTokenPermissions `json:"max_token_permissions,omitempty"`
	// AllowedCrossRepoIDs is a list of specific repo IDs that can be accessed cross-repo
	AllowedCrossRepoIDs []int64 `json:"allowed_cross_repo_ids,omitempty"`
}

// GetUserActionsConfig loads the UserActionsConfig for a user or organization from user settings
// It returns a default config if no setting is found
func GetUserActionsConfig(ctx context.Context, userID int64) (*UserActionsConfig, error) {
	var def UserActionsConfig
	ret, err := user_model.GetUserSettingJSON(ctx, userID, user_model.SettingsKeyActionsConfig, def)
	if err == nil {
		if ret.TokenPermissionMode == "" {
			ret.TokenPermissionMode = repo_model.ActionsTokenPermissionModePermissive
		}
	}
	return &ret, err
}

// SetUserActionsConfig saves the UserActionsConfig for a user or organization to user settings
func SetUserActionsConfig(ctx context.Context, userID int64, cfg *UserActionsConfig) error {
	return user_model.SetUserSettingJSON(ctx, userID, user_model.SettingsKeyActionsConfig, cfg)
}

// GetDefaultTokenPermissions returns the default token permissions by its TokenPermissionMode.
func (cfg *UserActionsConfig) GetDefaultTokenPermissions() repo_model.ActionsTokenPermissions {
	mode := cfg.TokenPermissionMode
	if mode == "" {
		mode = repo_model.ActionsTokenPermissionModePermissive
	}
	switch mode {
	case repo_model.ActionsTokenPermissionModeRestricted:
		return repo_model.GetRestrictedPermissions()
	case repo_model.ActionsTokenPermissionModePermissive:
		return repo_model.ActionsTokenPermissions{
			Code:         perm.AccessModeWrite,
			Issues:       perm.AccessModeWrite,
			PullRequests: perm.AccessModeWrite,
			Packages:     perm.AccessModeWrite,
			Actions:      perm.AccessModeWrite,
			Wiki:         perm.AccessModeWrite,
			Releases:     perm.AccessModeWrite,
			Projects:     perm.AccessModeWrite,
		}
	default:
		return repo_model.ActionsTokenPermissions{
			Code:         perm.AccessModeNone,
			Issues:       perm.AccessModeNone,
			PullRequests: perm.AccessModeNone,
			Packages:     perm.AccessModeNone,
			Actions:      perm.AccessModeNone,
			Wiki:         perm.AccessModeNone,
			Releases:     perm.AccessModeNone,
			Projects:     perm.AccessModeNone,
		}
	}
}

// GetMaxTokenPermissions returns the maximum allowed permissions
func (cfg *UserActionsConfig) GetMaxTokenPermissions() repo_model.ActionsTokenPermissions {
	if cfg.MaxTokenPermissions != nil {
		return *cfg.MaxTokenPermissions
	}
	// Default max is write for everything
	return repo_model.ActionsTokenPermissions{
		Code:         perm.AccessModeWrite,
		Issues:       perm.AccessModeWrite,
		PullRequests: perm.AccessModeWrite,
		Packages:     perm.AccessModeWrite,
		Actions:      perm.AccessModeWrite,
		Wiki:         perm.AccessModeWrite,
		Releases:     perm.AccessModeWrite,
		Projects:     perm.AccessModeWrite,
	}
}

// ClampPermissions ensures that the given permissions don't exceed the maximum
func (cfg *UserActionsConfig) ClampPermissions(perms repo_model.ActionsTokenPermissions) repo_model.ActionsTokenPermissions {
	maxPerms := cfg.GetMaxTokenPermissions()
	return perms.ClampPermissions(maxPerms)
}
