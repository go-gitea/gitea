// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import "gitea.dev/modelmigration/base"

func AddScopeForAccessTokens(x base.EngineMigration) error {
	type AccessToken struct {
		Scope string
	}

	if err := x.Sync(new(AccessToken)); err != nil {
		return err
	}

	// all previous tokens have `all` and `sudo` scopes
	_, err := x.Exec("UPDATE access_token SET scope = ? WHERE scope IS NULL OR scope = ''", "all,sudo")
	return err
}
