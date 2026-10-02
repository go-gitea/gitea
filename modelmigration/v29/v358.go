// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

type userOrgArchived struct {
	IsArchived   bool               `xorm:"NOT NULL DEFAULT false"`
	ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT 0"`
}

func (userOrgArchived) TableName() string {
	return "user"
}

func AddOrgArchivedColumns(_ context.Context, x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(userOrgArchived))
	return err
}
