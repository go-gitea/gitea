// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

// ImmutableTag records a tag name used by an immutable release. It outlives both the release and the
// tag itself, so the name can never back another release or be pushed again.
type ImmutableTag struct {
	ID           int64              `xorm:"pk autoincr"`
	RepoID       int64              `xorm:"UNIQUE(s) NOT NULL"`
	LowerTagName string             `xorm:"UNIQUE(s) NOT NULL"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(ImmutableTag))
}

// AddImmutableTag locks a tag name permanently, locking an already locked name is a no-op.
func AddImmutableTag(ctx context.Context, repoID int64, tagName string) error {
	immutable, err := IsTagImmutable(ctx, repoID, tagName)
	if err != nil || immutable {
		return err
	}
	return db.Insert(ctx, &ImmutableTag{RepoID: repoID, LowerTagName: strings.ToLower(tagName)})
}

// IsTagImmutable reports whether the tag name was used by an immutable release.
func IsTagImmutable(ctx context.Context, repoID int64, tagName string) (bool, error) {
	return db.Exist[ImmutableTag](ctx, builder.Eq{"repo_id": repoID, "lower_tag_name": strings.ToLower(tagName)})
}
