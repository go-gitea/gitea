// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

func CreateRepoSettingsTable(x *xorm.Engine) error {
	type RepoSetting db.ResourceSetting // to generate a table named repo_setting
	return x.Sync2(new(RepoSetting))
}
