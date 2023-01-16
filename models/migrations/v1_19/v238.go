// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	auth_models "code.gitea.io/gitea/models/auth"

	"xorm.io/xorm"
)

func AddScopeForAccessTokens(x *xorm.Engine) error {
	type AccessToken struct {
		Scope auth_models.AccessTokenScope
	}

	if err := x.Sync(new(AccessToken)); err != nil {
		return err
	}

	// all previous tokens have `all` and `sudo` scopes
	_, err := x.Exec("UPDATE access_token SET scope = ? WHERE scope IS NULL OR scope = ''", auth_models.AccessTokenScopeAll+","+auth_models.AccessTokenScopeSudo)
	return err
}
