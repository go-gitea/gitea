// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	api "code.gitea.io/gitea/modules/structs"
)

// ToEnvironment converts ActionEnvironment to API format
func ToEnvironment(ctx context.Context, env *actions_model.ActionEnvironment) (*api.Environment, error) {
	result := &api.Environment{
		ID:              env.ID,
		Name:            env.Name,
		Description:     env.Description,
		ExternalURL:     env.ExternalURL,
		ProtectionRules: env.ProtectionRules,
		CreatedAt:       env.CreatedUnix.AsTime(),
		UpdatedAt:       env.UpdatedUnix.AsTime(),
	}

	// Load and convert the creator if available
	if env.CreatedByID > 0 {
		if err := env.LoadCreatedBy(ctx); err == nil && env.CreatedBy != nil {
			result.CreatedBy = ToUser(ctx, env.CreatedBy, nil)
		}
	}

	return result, nil
}