// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/unit"

	"xorm.io/builder"
)

// IsImmutableReleasesEnabled reports whether newly published releases of this repository are locked.
func (repo *Repository) IsImmutableReleasesEnabled(ctx context.Context) bool {
	return repo.MustGetUnit(ctx, unit.TypeReleases).ReleasesConfig().ImmutableReleases
}

// ImmutableTag records a tag name used by an immutable release. It outlives the release, the tag and
// the repository itself, so the name can never back another release or be pushed again. The repository
// is recorded twice, by id and by path, and matching either claims the name: the id covers renames and
// transfers while the repository lives, the path covers one recreated where it used to be.
type ImmutableTag struct {
	ID            int64  `xorm:"pk autoincr"`
	RepoID        int64  `xorm:"UNIQUE(r) NOT NULL"`
	OwnerID       int64  `xorm:"INDEX(s) NOT NULL"`
	LowerRepoName string `xorm:"INDEX(s) NOT NULL"`
	LowerTagName  string `xorm:"INDEX(s) UNIQUE(r) NOT NULL"`
}

func init() {
	db.RegisterModel(new(ImmutableTag))
}

// AddImmutableTag claims a tag name permanently.
func AddImmutableTag(ctx context.Context, repo *Repository, tagName string) error {
	return db.Insert(ctx, &ImmutableTag{
		RepoID:        repo.ID,
		OwnerID:       repo.OwnerID,
		LowerRepoName: repo.LowerName,
		LowerTagName:  strings.ToLower(tagName),
	})
}

// StampImmutableTagPath refreshes the path recorded at claim time, which rename and transfer leave
// stale. Only deletion needs it, because until then the repository id claims the name.
func StampImmutableTagPath(ctx context.Context, repo *Repository) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repo.ID).
		Cols("owner_id", "lower_repo_name").
		Update(&ImmutableTag{OwnerID: repo.OwnerID, LowerRepoName: repo.LowerName})
	return err
}

// IsTagImmutable reports whether the tag name was used by an immutable release of this repository
// or of an earlier repository at the same path.
func IsTagImmutable(ctx context.Context, repo *Repository, tagName string) (bool, error) {
	return db.Exist[ImmutableTag](ctx, builder.Eq{"lower_tag_name": strings.ToLower(tagName)}.And(
		builder.Eq{"repo_id": repo.ID}.Or(
			builder.Eq{"owner_id": repo.OwnerID, "lower_repo_name": repo.LowerName}),
	))
}
