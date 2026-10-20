// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddReleaseIDToReaction(_ctx context.Context, x base.EngineMigration) error {
	type Reaction struct {
		ID               int64              `xorm:"pk autoincr"`
		Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
		IssueID          int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		CommentID        int64              `xorm:"INDEX UNIQUE(s)"`
		ReleaseID        int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
		UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
		OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
	}
	return x.Sync(new(Reaction))
}
