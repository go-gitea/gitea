// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
)

// GetOrgActionsConfig loads the ActionsConfig for an organization from user settings
// It returns a default config if no setting is found
func GetOrgActionsConfig(ctx context.Context, orgID int64) (*repo_model.ActionsConfig, error) {
	val, err := user_model.GetUserSetting(ctx, orgID, "actions.config")
	if err != nil {
		return nil, err
	}

	cfg := &repo_model.ActionsConfig{}
	if val == "" {
		// Return defaults if no config exists
		cfg.AllowCrossRepoAccess = true
		return cfg, nil
	}

	if err := json.Unmarshal([]byte(val), cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SetOrgActionsConfig saves the ActionsConfig for an organization to user settings
func SetOrgActionsConfig(ctx context.Context, orgID int64, cfg *repo_model.ActionsConfig) error {
	bs, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return user_model.SetUserSetting(ctx, orgID, "actions.config", string(bs))
}
