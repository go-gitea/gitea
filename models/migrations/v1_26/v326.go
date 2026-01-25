// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddActionsTokenPermissions(x *xorm.Engine) error {
	type ActionTokenPermissions struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"UNIQUE(repo_org) INDEX NOT NULL"`
		OrgID  int64 `xorm:"UNIQUE(repo_org) INDEX NOT NULL DEFAULT 0"`

		DefaultPermissions     string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'read'"`
		ContentsPermission     string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
		IssuesPermission       string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
		PullRequestsPermission string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
		PackagesPermission     string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
		MetadataPermission     string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'read'"`
		ActionsPermission      string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
		OrganizationPermission string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
		NotificationPermission string `xorm:"VARCHAR(20) NOT NULL DEFAULT ''"`
	}

	type ActionTask struct {
		ID                int64              `xorm:"pk autoincr"`
		JobID             int64              `xorm:"index"`
		Attempt           int64
		RunnerID          int64              `xorm:"index"`
		Status            int                `xorm:"index"`
		Started           timeutil.TimeStamp `xorm:"index"`
		Stopped           timeutil.TimeStamp `xorm:"index(stopped_log_expired)"`
		RepoID            int64              `xorm:"index"`
		OwnerID           int64              `xorm:"index"`
		CommitSHA         string             `xorm:"index"`
		IsForkPullRequest bool
		TokenHash         string `xorm:"UNIQUE"`
		TokenSalt         string
		TokenLastEight    string `xorm:"index token_last_eight"`
		TokenScopes       string `xorm:"TEXT"` // NEW FIELD
		LogFilename       string
		LogInStorage      bool
		LogLength         int64
		LogSize           int64
		LogIndexes        []byte `xorm:"LONGBLOB"`
		LogExpired        bool   `xorm:"index(stopped_log_expired)"`
		Created           timeutil.TimeStamp `xorm:"created"`
		Updated           timeutil.TimeStamp `xorm:"updated index"`
	}

	if err := x.Sync(new(ActionTokenPermissions)); err != nil {
		return err
	}

	return x.Sync(new(ActionTask))
}
