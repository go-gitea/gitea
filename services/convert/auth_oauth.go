// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
)

// ToOauthProvider convert auth_model.Source≤ to api.AuthOauth2Option
func ToOauthProvider(ctx context.Context, provider *auth_model.Source) *api.AuthSourceOption {
	if provider == nil {
		return nil
	}

	return toOauthProvider(provider)
}

// ToOauthProviders convert list of auth_model.Source to list of api.AuthOauth2Option
func ToOauthProviders(ctx context.Context, provider []*auth_model.Source) []*api.AuthSourceOption {
	result := make([]*api.AuthSourceOption, len(provider))
	for i := range provider {
		result[i] = ToOauthProvider(ctx, provider[i])
	}
	return result
}

func toOauthProvider(provider *auth_model.Source) *api.AuthSourceOption {
	return &api.AuthSourceOption{
		ID:                 provider.ID,
		AuthenticationName: provider.Name,
		TypeName:           provider.Type.String(),

		IsActive:      provider.IsActive,
		IsSyncEnabled: provider.IsSyncEnabled,
	}
}
