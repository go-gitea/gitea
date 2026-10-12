// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

// Collaborator grants a user or a team access to a non-public advisory, exactly one of UserID and TeamID is set.
// Read-only collaborators, like the reporters of duplicates, can follow and comment but not edit the advisory.
type Collaborator struct {
	ID         int64 `xorm:"pk autoincr"`
	AdvisoryID int64 `xorm:"UNIQUE(s) NOT NULL"`
	UserID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	TeamID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	ReadOnly   bool  `xorm:"NOT NULL DEFAULT false"`
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

// AddCollaborator returns false if the user or team already collaborates, also with another access
func AddCollaborator(ctx context.Context, c *Collaborator) (bool, error) {
	has, err := db.Exist[Collaborator](ctx, c.identityCond())
	if err != nil || has {
		return false, err
	}
	return true, db.Insert(ctx, c)
}

// SetCollaboratorWritable returns false if the user or team is no read-only collaborator
func SetCollaboratorWritable(ctx context.Context, c *Collaborator) (bool, error) {
	n, err := db.GetEngine(ctx).Where(c.identityCond().And(builder.Eq{"read_only": true})).Cols("read_only").Update(&Collaborator{ReadOnly: false})
	return n > 0, err
}

// RemoveCollaborator returns false if the user or team didn't collaborate
func RemoveCollaborator(ctx context.Context, c *Collaborator) (bool, error) {
	n, err := db.GetEngine(ctx).Where(c.identityCond()).Delete(new(Collaborator))
	return n > 0, err
}

// RemoveReadOnlyCollaborator keeps a user who has been granted write access in the meantime
func RemoveReadOnlyCollaborator(ctx context.Context, advisoryID, userID int64) (bool, error) {
	n, err := db.GetEngine(ctx).Where("advisory_id = ? AND user_id = ? AND read_only = ?", advisoryID, userID, true).Delete(new(Collaborator))
	return n > 0, err
}

// DeleteTeamCollaboratorsByRepoID is used when the repository leaves the organization of the teams
func DeleteTeamCollaboratorsByRepoID(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("team_id > 0").
		And(builder.In("advisory_id", builder.Select("id").From("security_advisory").Where(builder.Eq{"repo_id": repoID}))).
		Delete(new(Collaborator))
	return err
}

// DeleteUserCollaboratorsByOwnerID is used when the owner of the repositories blocks the user
func DeleteUserCollaboratorsByOwnerID(ctx context.Context, userID, ownerID int64) error {
	_, err := db.GetEngine(ctx).Where("user_id = ?", userID).
		And(builder.In("advisory_id", builder.Select("security_advisory.id").From("security_advisory").
			Join("INNER", "repository", "repository.id = security_advisory.repo_id").Where(builder.Eq{"repository.owner_id": ownerID}))).
		Delete(new(Collaborator))
	return err
}
