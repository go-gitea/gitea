// // Copyright 2020 The Gitea Authors. All rights reserved.
// // SPDX-License-Identifier: MIT

package convert

import (
	"context"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
)

// ToUser convert user_model.User to api.User
// if doer is set, private information is added if the doer has the permission to see it
func ToOauthProvider(ctx context.Context, provider *auth_model.Source) *api.AuthOauth2Option {
	if provider == nil {
		return nil
	}

	return toOauthProvider(provider)
}

// ToUsers convert list of user_model.User to list of api.User
func ToOauthProviders(ctx context.Context, provider []*auth_model.Source) []*api.AuthOauth2Option {
	result := make([]*api.AuthOauth2Option, len(provider))
	for i := range provider {
		result[i] = ToOauthProvider(ctx, provider[i])
	}
	return result
}

func toOauthProvider(provider *auth_model.Source) *api.AuthOauth2Option {
	return &api.AuthOauth2Option{
		ID:                 provider.ID,
		AuthenticationName: provider.Name,
		Type:               provider.Type.Int(),
		TypeName:           provider.Type.String(),

		IsActive:      provider.IsActive,
		IsSyncEnabled: provider.IsSyncEnabled,
	}
}
