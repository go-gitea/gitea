// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"

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
	collaboration, _, err := db.Get[Collaboration](ctx, builder.Eq{"repo_id": repoID, "user_id": uid})
	return collaboration, err
}

// IsCollaborator check if a user is a collaborator of a repository
func IsCollaborator(ctx context.Context, repoID, userID int64) (bool, error) {
	return db.Exist[Collaboration](ctx, builder.Eq{"repo_id": repoID, "user_id": userID})
}

func HasAccessToRepoCodeUnit(ctx context.Context, repo *Repository, userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	teamMember, err := db.GetEngine(ctx).Table("team_user").
		Join("INNER", "team_repo", "team_repo.team_id = team_user.team_id").
		Join("INNER", "team", "team.id = team_user.team_id").
		Join("LEFT", "team_unit", "team_unit.team_id = team_user.team_id AND team_unit.`type` = ?", unit.TypeCode).
		Where("team_repo.repo_id = ?", repo.ID).
		And("team_user.uid = ?", userID).
		And(builder.Or(
			builder.Gt{"team.authorize": perm.AccessModeNone},
			builder.Gt{"team_unit.access_mode": perm.AccessModeNone},
		)).
		Exist()
	if err != nil {
		return false, err
	}
	if teamMember {
		return true, nil
	}

	return db.Exist[Collaboration](ctx, builder.Eq{"repo_id": repo.ID, "user_id": userID})
}
