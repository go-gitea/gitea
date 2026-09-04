// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/unit"
	"gitea.dev/modules/util"
)

// a pull mirror follows upstream refs, so it cannot promise a tag will never move
func (repo *Repository) IsImmutableReleasesEnabled(ctx context.Context) bool {
	return !repo.IsMirror && repo.MustGetUnit(ctx, unit.TypeReleases).ReleasesConfig().ImmutableReleases
}

// ImmutableTag permanently claims a tag name at the path it was published at.
type ImmutableTag struct {
	ID             int64  `xorm:"pk autoincr"`
	LowerOwnerName string `xorm:"UNIQUE(s) NOT NULL"` // an identity, so matched folded
	LowerRepoName  string `xorm:"UNIQUE(s) NOT NULL"` // an identity, so matched folded
	TagName        string `xorm:"UNIQUE(s) NOT NULL"` // a git ref, so matched exactly
}

func init() {
	db.RegisterModel(new(ImmutableTag))
}

// LockRelease must run in the transaction writing the release, so row and claim commit together.
func LockRelease(ctx context.Context, repo *Repository, rel *Release) error {
	if rel.IsDraft || rel.IsTag || !repo.IsImmutableReleasesEnabled(ctx) {
		return nil
	}
	rel.IsImmutable = true
	if rel.ID != 0 { // an existing row needs the flag written here, UpdateRelease never writes it
		affected, err := db.GetEngine(ctx).ID(rel.ID).Cols("is_immutable").Update(rel)
		if err != nil {
			return err
		}
		if affected == 0 { // deleted meanwhile, so claiming its name would be wrong
			return util.NewNotExistErrorf("release does not exist [id: %d]", rel.ID)
		}
	}
	return db.Insert(ctx, &ImmutableTag{
		LowerOwnerName: strings.ToLower(repo.OwnerName),
		LowerRepoName:  repo.LowerName,
		TagName:        rel.TagName,
	})
}

func IsTagImmutable(ctx context.Context, repo *Repository, tagName string) (bool, error) {
	return db.GetEngine(ctx).Exist(&ImmutableTag{
		LowerOwnerName: strings.ToLower(repo.OwnerName),
		LowerRepoName:  repo.LowerName,
		TagName:        tagName,
	})
}
