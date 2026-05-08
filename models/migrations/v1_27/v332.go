// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddGithubAppCredentialTable(ctx context.Context, x *xorm.Engine) error {
	type GithubAppCredential struct {
		ID                  int64              `xorm:"pk autoincr"`
		OwnerID             int64              `xorm:"INDEX NOT NULL"`
		Name                string             `xorm:"NOT NULL"`
		AppID               int64              `xorm:"NOT NULL"`
		InstallationID      int64              `xorm:"NOT NULL"`
		PrivateKeyEncrypted string             `xorm:"TEXT NOT NULL"`
		BaseURL             string             `xorm:"VARCHAR(255) NOT NULL DEFAULT 'https://api.github.com'"`
		CreatedUnix         timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(GithubAppCredential))
}
