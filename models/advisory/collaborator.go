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

// AddCollaborator returns false if the user (teamID 0) or team (userID 0) already collaborates with at least this access
func AddCollaborator(ctx context.Context, advisoryID, userID, teamID int64, readOnly bool) (bool, error) {
	existing := new(Collaborator)
	has, err := db.GetEngine(ctx).Where("advisory_id = ? AND user_id = ? AND team_id = ?", advisoryID, userID, teamID).Get(existing)
	if err != nil {
		return false, err
	} else if !has {
		return true, db.Insert(ctx, &Collaborator{AdvisoryID: advisoryID, UserID: userID, TeamID: teamID, ReadOnly: readOnly})
	} else if readOnly || !existing.ReadOnly {
		return false, nil
	}
	_, err = db.GetEngine(ctx).ID(existing.ID).Cols("read_only").Update(&Collaborator{ReadOnly: false})
	return true, err
}

// RemoveReadOnlyCollaborator keeps a user who has been granted write access in the meantime
func RemoveReadOnlyCollaborator(ctx context.Context, advisoryID, userID int64) (bool, error) {
	n, err := db.GetEngine(ctx).Where("advisory_id = ? AND user_id = ? AND read_only = ?", advisoryID, userID, true).Delete(new(Collaborator))
	return n > 0, err
}

// RemoveCollaborator returns false if the user (teamID 0) or team (userID 0) didn't collaborate
func RemoveCollaborator(ctx context.Context, advisoryID, userID, teamID int64) (bool, error) {
	n, err := db.GetEngine(ctx).Where("advisory_id = ? AND user_id = ? AND team_id = ?", advisoryID, userID, teamID).Delete(new(Collaborator))
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
