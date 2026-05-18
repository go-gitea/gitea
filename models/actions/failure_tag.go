// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ActionRunFailureTag is a repo-scoped tag describing a category of action run failure.
type ActionRunFailureTag struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"INDEX UNIQUE(repo_name) NOT NULL"`
	Name        string             `xorm:"VARCHAR(50) UNIQUE(repo_name) NOT NULL"`
	Color       string             `xorm:"VARCHAR(7) NOT NULL DEFAULT ''"` // #rrggbb
	Description string             `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (*ActionRunFailureTag) TableName() string { return "action_run_failure_tag" }

func init() {
	db.RegisterModel(new(ActionRunFailureTag))
}

func normalizeFailureTagName(name string) string {
	return strings.TrimSpace(name)
}

// ListRepoFailureTags returns all failure tags defined on a repository, ordered by name.
func ListRepoFailureTags(ctx context.Context, repoID int64) ([]*ActionRunFailureTag, error) {
	tags := make([]*ActionRunFailureTag, 0)
	return tags, db.GetEngine(ctx).Where("repo_id = ?", repoID).Asc("name").Find(&tags)
}

// GetFailureTagByID returns a tag by id, scoped to the given repo for safety.
func GetFailureTagByID(ctx context.Context, repoID, id int64) (*ActionRunFailureTag, error) {
	var tag ActionRunFailureTag
	has, err := db.GetEngine(ctx).Where("repo_id = ? AND id = ?", repoID, id).Get(&tag)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("failure tag %d in repo %d: %w", id, repoID, util.ErrNotExist)
	}
	return &tag, nil
}

// CreateFailureTag inserts a new failure tag. Name is required and must be unique per repo.
func CreateFailureTag(ctx context.Context, tag *ActionRunFailureTag) error {
	tag.Name = normalizeFailureTagName(tag.Name)
	if tag.Name == "" {
		return util.NewInvalidArgumentErrorf("failure tag name is required")
	}
	_, err := db.GetEngine(ctx).Insert(tag)
	return err
}

// UpdateFailureTag mutates name/color/description of a tag.
func UpdateFailureTag(ctx context.Context, tag *ActionRunFailureTag) error {
	tag.Name = normalizeFailureTagName(tag.Name)
	if tag.Name == "" {
		return util.NewInvalidArgumentErrorf("failure tag name is required")
	}
	_, err := db.GetEngine(ctx).ID(tag.ID).Cols("name", "color", "description").Update(tag)
	return err
}

// DeleteFailureTag removes a tag and any analysis-tag links pointing at it.
func DeleteFailureTag(ctx context.Context, repoID, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("tag_id = ?", id).Delete(new(ActionRunAnalysisTag)); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).Where("repo_id = ? AND id = ?", repoID, id).Delete(new(ActionRunFailureTag))
		return err
	})
}
