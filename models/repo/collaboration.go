// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// Collaboration represent the relation between an individual and a repository.
type Collaboration struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	UserID      int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Mode        perm.AccessMode    `xorm:"DEFAULT 2 NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Collaboration))
}

// Collaborator represents a user with collaboration details.
type Collaborator struct {
	*user_model.User
	Collaboration *Collaboration
}

// GetCollaborators returns the collaborators for a repository
func GetCollaborators(ctx context.Context, repoID int64, listOptions db.ListOptions) ([]*Collaborator, error) {
	collaborations, err := getCollaborations(ctx, repoID, listOptions)
	if err != nil {
		return nil, fmt.Errorf("getCollaborations: %w", err)
	}

	collaborators := make([]*Collaborator, 0, len(collaborations))
	for _, c := range collaborations {
		user, err := user_model.GetUserByID(ctx, c.UserID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				log.Warn("Inconsistent DB: User: %d is listed as collaborator of %-v but does not exist", c.UserID, repoID)
				user = user_model.NewGhostUser()
			} else {
				return nil, err
			}
		}
		collaborators = append(collaborators, &Collaborator{
			User:          user,
			Collaboration: c,
		})
	}
	return collaborators, nil
}

// CountCollaborators returns total number of collaborators for a repository
func CountCollaborators(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Where("repo_id = ? ", repoID).Count(&Collaboration{})
}

// GetCollaboration get collaboration for a repository id with a user id
func GetCollaboration(ctx context.Context, repoID, uid int64) (*Collaboration, error) {
	collaboration := &Collaboration{
		RepoID: repoID,
		UserID: uid,
	}
	has, err := db.GetEngine(ctx).Get(collaboration)
	if !has {
		collaboration = nil
	}
	return collaboration, err
}

// IsCollaborator check if a user is a collaborator of a repository
func IsCollaborator(ctx context.Context, repoID, userID int64) (bool, error) {
	return db.GetEngine(ctx).Get(&Collaboration{RepoID: repoID, UserID: userID})
}

func getCollaborations(ctx context.Context, repoID int64, listOptions db.ListOptions) ([]*Collaboration, error) {
	if listOptions.Page == 0 {
		collaborations := make([]*Collaboration, 0, 8)
		return collaborations, db.GetEngine(ctx).Find(&collaborations, &Collaboration{RepoID: repoID})
	}

	e := db.GetEngine(ctx)

	e = db.SetEnginePagination(e, &listOptions)

	collaborations := make([]*Collaboration, 0, listOptions.PageSize)
	return collaborations, e.Find(&collaborations, &Collaboration{RepoID: repoID})
}

// ChangeCollaborationAccessMode sets new access mode for the collaboration.
func ChangeCollaborationAccessMode(ctx context.Context, repo *Repository, uid int64, mode perm.AccessMode) error {
	// Discard invalid input
	if mode <= perm.AccessModeNone || mode > perm.AccessModeOwner {
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)

		collaboration := &Collaboration{
			RepoID: repo.ID,
			UserID: uid,
		}
		has, err := e.Get(collaboration)
		if err != nil {
			return fmt.Errorf("get collaboration: %w", err)
		} else if !has {
			return nil
		}

		if collaboration.Mode == mode {
			return nil
		}
		collaboration.Mode = mode

		if _, err = e.
			ID(collaboration.ID).
			Cols("mode").
			Update(collaboration); err != nil {
			return fmt.Errorf("update collaboration: %w", err)
		} else if _, err = e.Exec("UPDATE access SET mode = ? WHERE user_id = ? AND repo_id = ?", mode, uid, repo.ID); err != nil {
			return fmt.Errorf("update access table: %w", err)
		}

		return nil
	})
}

// IsOwnerMemberCollaborator checks if a provided user is the owner, a collaborator or a member of a team in a repository
func IsOwnerMemberCollaborator(ctx context.Context, repo *Repository, userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	teamMember, err := db.GetEngine(ctx).Join("INNER", "team_repo", "team_repo.team_id = team_user.team_id").
		Join("INNER", "team_unit", "team_unit.team_id = team_user.team_id").
		Where("team_repo.repo_id = ?", repo.ID).
		And("team_unit.`type` = ?", unit.TypeCode).
		And("team_user.uid = ?", userID).Table("team_user").Exist()
	if err != nil {
		return false, err
	}
	if teamMember {
		return true, nil
	}

	return db.GetEngine(ctx).Get(&Collaboration{RepoID: repo.ID, UserID: userID})
}
