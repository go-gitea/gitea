// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
)

// GetUserActionsConfig loads the ActionsConfig for a user or organization from user settings
// It returns a default config if no setting is found
func GetUserActionsConfig(ctx context.Context, userID int64) (*repo_model.ActionsConfig, error) {
	val, err := user_model.GetUserSetting(ctx, userID, "actions.config")
	if err != nil {
		return nil, err
	}

	cfg := &repo_model.ActionsConfig{}
	if val == "" {
		// Return defaults if no config exists
		cfg.CrossRepoMode = repo_model.ActionsCrossRepoModeAll
		return cfg, nil
	}

	if err := json.Unmarshal([]byte(val), cfg); err != nil {
		return nil, err
	}

	// Normalize empty CrossRepoMode to the default (All)
	if cfg.CrossRepoMode == "" {
		cfg.CrossRepoMode = repo_model.ActionsCrossRepoModeAll
	}

	return cfg, nil
}

// SetUserActionsConfig saves the ActionsConfig for a user or organization to user settings
func SetUserActionsConfig(ctx context.Context, userID int64, cfg *repo_model.ActionsConfig) error {
	bs, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return user_model.SetUserSetting(ctx, userID, "actions.config", string(bs))
}
