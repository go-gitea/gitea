// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/unit"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// a pull mirror follows upstream refs, so it cannot promise a tag will never move
func (repo *Repository) IsImmutableReleasesEnabled(ctx context.Context) bool {
	return !repo.IsMirror && repo.MustGetUnit(ctx, unit.TypeReleases).ReleasesConfig().ImmutableReleases
}

var ErrImmutableTag = util.ErrorWrap(util.ErrUnprocessableContent, "tag_name was used by an immutable release")

// ImmutableTag permanently claims a tag name for its repository and for the path it was published at.
type ImmutableTag struct {
	ID             int64  `xorm:"pk autoincr"`
	RepoID         int64  `xorm:"INDEX(r) NOT NULL"`
	LowerOwnerName string `xorm:"UNIQUE(s) NOT NULL"`
	LowerRepoName  string `xorm:"UNIQUE(s) NOT NULL"`
	TagName        string `xorm:"UNIQUE(s) INDEX(r) NOT NULL"` // a git ref, so matched exactly, unlike the folded path
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
	if rel.ID != 0 { // UpdateRelease never writes the flag
		if _, err := db.GetEngine(ctx).ID(rel.ID).Cols("is_immutable").Update(rel); err != nil {
			return err
		}
	}
	return db.Insert(ctx, &ImmutableTag{
		RepoID:         repo.ID,
		LowerOwnerName: strings.ToLower(repo.OwnerName),
		LowerRepoName:  repo.LowerName,
		TagName:        rel.TagName,
	})
}

func immutableTagCond(repo *Repository) builder.Cond {
	return builder.Or(
		builder.Eq{"repo_id": repo.ID},
		builder.Eq{"lower_owner_name": strings.ToLower(repo.OwnerName), "lower_repo_name": repo.LowerName},
	)
}

func IsTagImmutable(ctx context.Context, repo *Repository, tagName string) (bool, error) {
	return db.Exist[ImmutableTag](ctx, immutableTagCond(repo).And(builder.Eq{"tag_name": tagName}))
}

func GetImmutableTagNames(ctx context.Context, repo *Repository) (container.Set[string], error) {
	var names []string
	if err := db.GetEngine(ctx).Table("immutable_tag").Where(immutableTagCond(repo)).Cols("tag_name").Find(&names); err != nil {
		return nil, err
	}
	return container.SetOf(names...), nil
}

// CheckImmutableTagConflicts rejects a repository arriving at a path where another repository claimed one of its tag names.
func CheckImmutableTagConflicts(ctx context.Context, repoID int64, ownerName, repoName string) error {
	conflict, err := db.Exist[ImmutableTag](ctx, builder.Eq{
		"lower_owner_name": strings.ToLower(ownerName),
		"lower_repo_name":  strings.ToLower(repoName),
	}.And(builder.Neq{"repo_id": repoID}, builder.In("tag_name",
		builder.Select("tag_name").From("`release`").Where(builder.Eq{"repo_id": repoID}),
	)))
	if err != nil || !conflict {
		return err
	}
	return ErrImmutableTag
}
