// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
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
	e := db.GetEngine(ctx)
	collaborations, err := getCollaborations(e, repoID, listOptions)
	if err != nil {
		return nil, fmt.Errorf("getCollaborations: %v", err)
	}

	collaborators := make([]*Collaborator, 0, len(collaborations))
	for _, c := range collaborations {
		user, err := user_model.GetUserByIDEngine(e, c.UserID)
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
func CountCollaborators(repoID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("repo_id = ? ", repoID).Count(&Collaboration{})
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

func getCollaborations(e db.Engine, repoID int64, listOptions db.ListOptions) ([]*Collaboration, error) {
	if listOptions.Page == 0 {
		collaborations := make([]*Collaboration, 0, 8)
		return collaborations, e.Find(&collaborations, &Collaboration{RepoID: repoID})
	}

	e = db.SetEnginePagination(e, &listOptions)

	collaborations := make([]*Collaboration, 0, listOptions.PageSize)
	return collaborations, e.Find(&collaborations, &Collaboration{RepoID: repoID})
}

// ChangeCollaborationAccessMode sets new access mode for the collaboration.
func ChangeCollaborationAccessModeCtx(ctx context.Context, repo *Repository, uid int64, mode perm.AccessMode) error {
	// Discard invalid input
	if mode <= perm.AccessModeNone || mode > perm.AccessModeOwner {
		return nil
	}

	e := db.GetEngine(ctx)

	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: uid,
	}
	has, err := e.Get(collaboration)
	if err != nil {
		return fmt.Errorf("get collaboration: %v", err)
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
		return fmt.Errorf("update collaboration: %v", err)
	} else if _, err = e.Exec("UPDATE access SET mode = ? WHERE user_id = ? AND repo_id = ?", mode, uid, repo.ID); err != nil {
		return fmt.Errorf("update access table: %v", err)
	}

	return nil
}

// ChangeCollaborationAccessMode sets new access mode for the collaboration.
func ChangeCollaborationAccessMode(repo *Repository, uid int64, mode perm.AccessMode) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := ChangeCollaborationAccessModeCtx(ctx, repo, uid, mode); err != nil {
		return err
	}

	return committer.Commit()
}
