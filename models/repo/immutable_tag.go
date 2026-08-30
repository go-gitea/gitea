// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/models/unit"

	"xorm.io/builder"
)

func (repo *Repository) IsImmutableReleasesEnabled(ctx context.Context) bool {
	return repo.MustGetUnit(ctx, unit.TypeReleases).ReleasesConfig().ImmutableReleases
}

// ImmutableTag claims a tag name for good, outliving the release, the tag and the repository. The
// path is only filled in once the repository is deleted, so that a repository recreated there
// inherits the claim, while a live repository is matched by id and can be renamed freely.
type ImmutableTag struct {
	ID            int64  `xorm:"pk autoincr"`
	RepoID        int64  `xorm:"UNIQUE(r) NOT NULL"`
	OwnerID       int64  `xorm:"INDEX(s) NOT NULL"`
	LowerRepoName string `xorm:"INDEX(s) NOT NULL"`           // a repository name is an identity, so it is matched folded
	TagName       string `xorm:"INDEX(s) UNIQUE(r) NOT NULL"` // a tag name is a git ref, so it is matched exactly
}

func init() {
	db.RegisterModel(new(ImmutableTag))
}

// LockRelease claims the tag name of a release becoming published. Must run inside the transaction
// that writes the release, so the row and its claim commit together.
func LockRelease(ctx context.Context, repo *Repository, rel *Release) error {
	if rel.IsDraft || rel.IsTag || !repo.IsImmutableReleasesEnabled(ctx) {
		return nil
	}
	rel.IsImmutable = true
	return db.Insert(ctx, &ImmutableTag{RepoID: repo.ID, TagName: rel.TagName})
}

// StampImmutableTagPath records the path a deleted repository ended at, so its claims keep applying
// there. Until then the repository is claimed by id, which rename and transfer leave alone.
func StampImmutableTagPath(ctx context.Context, repo *Repository) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repo.ID).
		Cols("owner_id", "lower_repo_name").
		Update(&ImmutableTag{OwnerID: repo.OwnerID, LowerRepoName: repo.LowerName})
	return err
}

// IsTagImmutable also matches a claim left behind by a deleted repository at the same path.
func IsTagImmutable(ctx context.Context, repo *Repository, tagName string) (bool, error) {
	return db.Exist[ImmutableTag](ctx, builder.Eq{"tag_name": tagName}.And(
		builder.Eq{"repo_id": repo.ID}.Or(
			builder.Eq{"owner_id": repo.OwnerID, "lower_repo_name": repo.LowerName}),
	))
}
