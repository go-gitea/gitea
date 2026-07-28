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

// ImmutableTag records a tag name used by an immutable release. It outlives the release, the tag and
// the repository itself, so the name can never back another release or be pushed again.
// The repository is recorded twice: by id so the lock follows renames and transfers, and by owner
// and name so a repository recreated at the same path inherits it. Matching either one is enough.
type ImmutableTag struct {
	ID            int64              `xorm:"pk autoincr"`
	RepoID        int64              `xorm:"INDEX NOT NULL"`
	OwnerID       int64              `xorm:"UNIQUE(s) NOT NULL"`
	LowerRepoName string             `xorm:"UNIQUE(s) NOT NULL"`
	LowerTagName  string             `xorm:"UNIQUE(s) NOT NULL"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(ImmutableTag))
}

// AddImmutableTag locks a tag name permanently, locking an already locked name is a no-op.
func AddImmutableTag(ctx context.Context, repo *Repository, tagName string) error {
	immutable, err := IsTagImmutable(ctx, repo, tagName)
	if err != nil || immutable {
		return err
	}
	return db.Insert(ctx, &ImmutableTag{
		RepoID:        repo.ID,
		OwnerID:       repo.OwnerID,
		LowerRepoName: strings.ToLower(repo.Name),
		LowerTagName:  strings.ToLower(tagName),
	})
}

// IsTagImmutable reports whether the tag name was used by an immutable release of this repository
// or of an earlier repository at the same path.
func IsTagImmutable(ctx context.Context, repo *Repository, tagName string) (bool, error) {
	return db.Exist[ImmutableTag](ctx, builder.Eq{"lower_tag_name": strings.ToLower(tagName)}.And(
		builder.Eq{"repo_id": repo.ID}.Or(
			builder.Eq{"owner_id": repo.OwnerID, "lower_repo_name": strings.ToLower(repo.Name)}),
	))
}
