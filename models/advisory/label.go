// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

type LabelLink struct {
	ID         int64 `xorm:"pk autoincr"`
	AdvisoryID int64 `xorm:"UNIQUE(s) NOT NULL"`
	LabelID    int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
}

func init() {
	db.RegisterModel(new(LabelLink))
}

func (LabelLink) TableName() string {
	return "security_advisory_label"
}

// DeleteLabelLinks keeps links if a delete request named another owner's label.
func DeleteLabelLinks(ctx context.Context, labelID int64) error {
	_, err := db.GetEngine(ctx).Where("label_id = ?", labelID).
		And(builder.NotIn("label_id", builder.Select("id").From("label").Where(builder.Eq{"id": labelID}))).
		Delete(new(LabelLink))
	return err
}

func DeleteOrgLabelLinksByRepoID(ctx context.Context, repoID, orgID int64) error {
	_, err := db.GetEngine(ctx).
		Where(builder.In("advisory_id", builder.Select("id").From("security_advisory").Where(builder.Eq{"repo_id": repoID}))).
		And(builder.In("label_id", builder.Select("id").From("label").Where(builder.Eq{"org_id": orgID}))).
		Delete(new(LabelLink))
	return err
}
