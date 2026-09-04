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

func (repo *Repository) IsImmutableReleasesEnabled(ctx context.Context) bool {
	return repo.MustGetUnit(ctx, unit.TypeReleases).ReleasesConfig().ImmutableReleases
}

// ImmutableTag claims a tag name at the path it was published at, for good. It outlives the release,
// the tag and the repository, so that whatever is created at that path later inherits the claim.
type ImmutableTag struct {
	ID             int64  `xorm:"pk autoincr"`
	LowerOwnerName string `xorm:"UNIQUE(s) NOT NULL"` // an owner name is an identity, so it is matched folded
	LowerRepoName  string `xorm:"UNIQUE(s) NOT NULL"` // a repository name is an identity, so it is matched folded
	TagName        string `xorm:"UNIQUE(s) NOT NULL"` // a tag name is a git ref, so it is matched exactly
}

func init() {
	db.RegisterModel(new(ImmutableTag))
}

// LockRelease claims the tag name of a release becoming published. Must run inside the transaction
// that writes the release, so the row and its claim commit together.
func LockRelease(ctx context.Context, repo *Repository, rel *Release) error {
	// a pull mirror follows upstream refs, so it cannot promise a tag will never move
	if rel.IsDraft || rel.IsTag || repo.IsMirror || !repo.IsImmutableReleasesEnabled(ctx) {
		return nil
	}
	rel.IsImmutable = true
	if rel.ID != 0 { // an existing row needs the flag written here, UpdateRelease never writes it
		if _, err := db.GetEngine(ctx).ID(rel.ID).Cols("is_immutable").Update(rel); err != nil {
			return err
		}
	}
	return db.Insert(ctx, &ImmutableTag{
		LowerOwnerName: strings.ToLower(repo.OwnerName),
		LowerRepoName:  repo.LowerName,
		TagName:        rel.TagName,
	})
}

func IsTagImmutable(ctx context.Context, repo *Repository, tagName string) (bool, error) {
	return db.Exist[ImmutableTag](ctx, builder.Eq{
		"lower_owner_name": strings.ToLower(repo.OwnerName),
		"lower_repo_name":  repo.LowerName,
		"tag_name":         tagName,
	})
}
