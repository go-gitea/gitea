// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	auth_models "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
)

func addScopeForAccessTokens(x *xorm.Engine) error {
	type AccessTokenWithDefaultScope struct {
		ID             int64 `xorm:"pk autoincr"`
		UID            int64 `xorm:"INDEX"`
		Name           string
		Token          string `xorm:"-"`
		TokenHash      string `xorm:"UNIQUE"` // sha256 of token
		TokenSalt      string
		TokenLastEight string                       `xorm:"token_last_eight"`
		Scope          auth_models.AccessTokenScope `xorm:"NOT NULL DEFAULT 'all'"`

		CreatedUnix       timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix       timeutil.TimeStamp `xorm:"INDEX updated"`
		HasRecentActivity bool               `xorm:"-"`
		HasUsed           bool               `xorm:"-"`
	}

	err := x.Sync(new(AccessTokenWithDefaultScope))
	if err != nil {
		return err
	}

	// remove default 'all'
	return x.Sync(new(auth_models.AccessToken))
}
