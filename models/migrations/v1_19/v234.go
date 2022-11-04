// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_19 //nolint

import (
	auth_models "code.gitea.io/gitea/models/auth"

	"xorm.io/xorm"
)

func AddScopeForAccessTokens(x *xorm.Engine) error {
	err := addScopeField(x)
	if err != nil {
		return err
	}

	// remove default 'all' for scope
	return removeDefaultAll(x)
}

func addScopeField(x *xorm.Engine) error {
	type AccessToken struct {
		Scope auth_models.AccessTokenScope `xorm:"NOT NULL DEFAULT 'all'"`
	}
	return x.Sync(new(AccessToken))
}

func removeDefaultAll(x *xorm.Engine) error {
	type AccessToken struct {
		Scope auth_models.AccessTokenScope
	}
	return x.Sync(new(AccessToken))
}
