// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func AddGithubAppCredentialTable(x db.EngineMigration) error {
	type GithubAppCredential struct {
		ID                  int64              `xorm:"pk autoincr"`
		OwnerID             int64              `xorm:"INDEX NOT NULL"`
		Name                string             `xorm:"NOT NULL"`
		ClientID            string             `xorm:"NOT NULL"`
		InstallationID      int64              `xorm:"NOT NULL"`
		PrivateKeyEncrypted string             `xorm:"TEXT NOT NULL"`
		BaseURL             string             `xorm:"VARCHAR(255) NOT NULL DEFAULT 'https://api.github.com'"`
		CreatedUnix         timeutil.TimeStamp `xorm:"created"`
		LastUsedUnix        timeutil.TimeStamp `xorm:"last_used_unix"`
	}

	return x.Sync(new(GithubAppCredential))
}
