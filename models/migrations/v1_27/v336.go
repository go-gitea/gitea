// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
)

func AddGithubAppCredentialIDToMirror(x db.EngineMigration) error {
	type Mirror struct {
		GithubAppCredentialID int64 `xorm:"github_app_credential_id DEFAULT 0"`
	}

	return x.Sync(new(Mirror))
}
