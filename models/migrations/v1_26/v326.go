// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/json"

	"xorm.io/xorm"
)

func SetDefaultAllowMaintainerEdit(x *xorm.Engine) error {
	type RepoUnit struct {
		ID     int64
		Config string
	}

	var units []RepoUnit
	// type = 3 is TypePullRequests
	err := x.Table("repo_unit").Where("`type` = 3").Find(&units)
	if err != nil {
		return err
	}

	for _, unit := range units {
		if unit.Config == "" {
			continue
		}

		var cfg map[string]any
		if err := json.Unmarshal([]byte(unit.Config), &cfg); err != nil {
			return err
		}

		cfg["DefaultAllowMaintainerEdit"] = true
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}

		_, err = x.Exec("UPDATE `repo_unit` SET `config` = ? WHERE `id` = ?", string(data), unit.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
