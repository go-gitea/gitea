// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

type Collaborator struct {
	ID         int64 `xorm:"pk autoincr"`
	AdvisoryID int64 `xorm:"UNIQUE(s) NOT NULL"`
	UserID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	TeamID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	ReadOnly   bool  `xorm:"NOT NULL DEFAULT false"` // Only reporters of duplicates that share the original.
}

func init() {
	db.RegisterModel(new(Collaborator))
}

func (Collaborator) TableName() string {
	return "security_advisory_collaborator"
}

func (c *Collaborator) identityCond() builder.Cond {
	return builder.Eq{"advisory_id": c.AdvisoryID, "user_id": c.UserID, "team_id": c.TeamID}
}

func AddCollaborator(ctx context.Context, collaborator *Collaborator) (bool, error) {
	has, err := db.Exist[Collaborator](ctx, collaborator.identityCond())
	if err != nil || has {
		return false, err
	}
	return true, db.Insert(ctx, collaborator)
}

func SetCollaboratorWritable(ctx context.Context, collaborator *Collaborator) (bool, error) {
	updated, err := db.GetEngine(ctx).Where(collaborator.identityCond().And(builder.Eq{"read_only": true})).Cols("read_only").Update(&Collaborator{})
	return updated > 0, err
}

// RevokeSharedOriginal keeps the access while another duplicate of the reporter shares the same original
func RevokeSharedOriginal(ctx context.Context, duplicate *Advisory) (bool, error) {
	if !duplicate.ShareOriginal {
		return false, nil
	}
	sharedByAnother := builder.Eq{"duplicate_of_id": duplicate.DuplicateOfID, "reporter_id": duplicate.ReporterID, "share_original": true}.And(builder.Neq{"id": duplicate.ID})
	if has, err := db.Exist[Advisory](ctx, sharedByAnother); err != nil || has {
		return false, err
	}
	deleted, err := db.GetEngine(ctx).Where(builder.Eq{"advisory_id": duplicate.DuplicateOfID, "user_id": duplicate.ReporterID, "read_only": true}).Delete(new(Collaborator))
	return deleted > 0, err
}

func RemoveCollaborator(ctx context.Context, collaborator *Collaborator) (bool, error) {
	deleted, err := db.GetEngine(ctx).Where(collaborator.identityCond()).Delete(new(Collaborator))
	return deleted > 0, err
}

func DeleteTeamCollaboratorsByRepoID(ctx context.Context, repoID int64) error {
	return deleteTeamCollaborators(ctx, repoID, builder.Gt{"team_id": 0})
}

func DeleteTeamCollaboratorByRepoID(ctx context.Context, teamID, repoID int64) error {
	return deleteTeamCollaborators(ctx, repoID, builder.Eq{"team_id": teamID})
}

func deleteTeamCollaborators(ctx context.Context, repoID int64, teamCond builder.Cond) error {
	_, err := db.GetEngine(ctx).Where(teamCond).
		And(builder.In("advisory_id", builder.Select("id").From("security_advisory").Where(builder.Eq{"repo_id": repoID}))).
		Delete(new(Collaborator))
	return err
}

func DeleteUserCollaboratorsByOwnerID(ctx context.Context, userID, ownerID int64) error {
	_, err := db.GetEngine(ctx).Where("user_id = ?", userID).
		And(builder.In("advisory_id", builder.Select("security_advisory.id").From("security_advisory").
			Join("INNER", "repository", "repository.id = security_advisory.repo_id").Where(builder.Eq{"repository.owner_id": ownerID}))).
		Delete(new(Collaborator))
	return err
}
