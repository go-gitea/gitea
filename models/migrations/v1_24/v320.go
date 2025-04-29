// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"code.gitea.io/gitea/modules/json"

	"xorm.io/xorm"
)

func MigrateSkipTwoFactor(x *xorm.Engine) error {
	type LoginSource struct {
		TwoFactorPolicy string `xorm:"two_factor_policy NOT NULL DEFAULT ''"`
	}
	_, err := x.SyncWithOptions(
		xorm.SyncOptions{
			IgnoreConstrains: true,
			IgnoreIndices:    true,
		},
		new(LoginSource),
	)
	if err != nil {
		return err
	}

	type LoginSourceSimple struct {
		ID  int64
		Cfg string
	}

	var loginSources []LoginSourceSimple
	err = x.Table("login_source").Find(&loginSources)
	if err != nil {
		return err
	}

	for _, source := range loginSources {
		if source.Cfg == "" {
			continue
		}

		var cfg map[string]any
		err = json.Unmarshal([]byte(source.Cfg), &cfg)
		if err != nil {
			return err
		}

		if cfg["SkipLocalTwoFA"] == true {
			_, err = x.Exec("UPDATE login_source SET two_factor_policy = 'skip' WHERE id = ?", source.ID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
