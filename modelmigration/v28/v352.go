// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddTokenToDeployKey(ctx context.Context, x base.EngineMigration) error {
	// Drop the old UNIQUE(s) index on (key_id, repo_id). Every token row carries key
	// id 0, so the pair can no longer be unique. AddDeployKey still checks it in code.
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "deploy_key")
	if err != nil {
		return err
	}
	if idx, ok := indexes["s"]; ok {
		if _, err := x.Exec(x.Dialect().DropIndexSQL("deploy_key", idx)); err != nil {
			return err
		}
	}

	type DeployKey struct {
		KeyID     int64  `xorm:"INDEX"`
		RepoID    int64  `xorm:"INDEX"`
		KeyType   int    `xorm:"NOT NULL DEFAULT 1"` // every existing row is an SSH key
		TokenHash string `xorm:"INDEX"`
	}
	_, err = x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true, // the bean only describes the new columns
	}, new(DeployKey))
	return err
}
