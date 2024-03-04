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
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
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

type FindCollaborationOptions struct {
	db.ListOptions
	RepoID         int64
	RepoOwnerID    int64
	CollaboratorID int64
}

func (opts *FindCollaborationOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"collaboration.repo_id": opts.RepoID})
	}
	if opts.RepoOwnerID != 0 {
		cond = cond.And(builder.Eq{"repository.owner_id": opts.RepoOwnerID})
	}
	if opts.CollaboratorID != 0 {
		cond = cond.And(builder.Eq{"collaboration.user_id": opts.CollaboratorID})
	}
	return cond
}

func (opts *FindCollaborationOptions) ToJoins() []db.JoinFunc {
	if opts.RepoOwnerID != 0 {
		return []db.JoinFunc{
			func(e db.Engine) error {
				e.Join("INNER", "repository", "repository.id = collaboration.repo_id")
				return nil
			},
		}
	}
	return nil
}

// GetCollaborators returns the collaborators for a repository
func GetCollaborators(ctx context.Context, opts *FindCollaborationOptions) ([]*Collaborator, int64, error) {
	collaborations, total, err := db.FindAndCount[Collaboration](ctx, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("db.FindAndCount[Collaboration]: %w", err)
	}

	collaborators := make([]*Collaborator, 0, len(collaborations))
	userIDs := make([]int64, 0, len(collaborations))
	for _, c := range collaborations {
		userIDs = append(userIDs, c.UserID)
	}

	usersMap := make(map[int64]*user_model.User)
	if err := db.GetEngine(ctx).In("id", userIDs).Find(&usersMap); err != nil {
		return nil, 0, fmt.Errorf("Find users map by user ids: %w", err)
	}

	for _, c := range collaborations {
		u := usersMap[c.UserID]
		if u == nil {
			u = user_model.NewGhostUser()
		}
		collaborators = append(collaborators, &Collaborator{
			User:          u,
			Collaboration: c,
		})
	}
	return collaborators, total, nil
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
