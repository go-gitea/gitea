// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// GetUserActionsConfig loads the ActionsConfig for a user or organization from user settings
// It returns a default config if no setting is found
func GetUserActionsConfig(ctx context.Context, userID int64) (*repo_model.ActionsConfig, error) {
	var def repo_model.ActionsConfig
	ret, err := user_model.GetUserSettingJSON(ctx, userID, user_model.SettingsKeyActionsConfig, def)
	return &ret, err
}

// SetUserActionsConfig saves the ActionsConfig for a user or organization to user settings
func SetUserActionsConfig(ctx context.Context, userID int64, cfg *repo_model.ActionsConfig) error {
	return user_model.SetUserSettingJSON(ctx, userID, user_model.SettingsKeyActionsConfig, cfg)
}
