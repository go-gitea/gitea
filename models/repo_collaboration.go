// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
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

func addCollaborator(ctx context.Context, repo *repo_model.Repository, u *user_model.User) error {
	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: u.ID,
	}
	e := db.GetEngine(ctx)

	has, err := e.Get(collaboration)
	if err != nil {
		return err
	} else if has {
		return nil
	}
	collaboration.Mode = perm.AccessModeWrite

	if _, err = e.InsertOne(collaboration); err != nil {
		return err
	}

	return recalculateUserAccess(ctx, repo, u.ID)
}

// AddCollaborator adds new collaboration to a repository with default access mode.
func AddCollaborator(repo *repo_model.Repository, u *user_model.User) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := addCollaborator(ctx, repo, u); err != nil {
		return err
	}

	return committer.Commit()
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

// Collaborator represents a user with collaboration details.
type Collaborator struct {
	*user_model.User
	Collaboration *Collaboration
}

func getCollaborators(e db.Engine, repoID int64, listOptions db.ListOptions) ([]*Collaborator, error) {
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

// GetCollaborators returns the collaborators for a repository
func GetCollaborators(repoID int64, listOptions db.ListOptions) ([]*Collaborator, error) {
	return getCollaborators(db.GetEngine(db.DefaultContext), repoID, listOptions)
}

// CountCollaborators returns total number of collaborators for a repository
func CountCollaborators(repoID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("repo_id = ? ", repoID).Count(&Collaboration{})
}

func getCollaboration(e db.Engine, repoID, uid int64) (*Collaboration, error) {
	collaboration := &Collaboration{
		RepoID: repoID,
		UserID: uid,
	}
	has, err := e.Get(collaboration)
	if !has {
		collaboration = nil
	}
	return collaboration, err
}

func isCollaborator(e db.Engine, repoID, userID int64) (bool, error) {
	return e.Get(&Collaboration{RepoID: repoID, UserID: userID})
}

// IsCollaborator check if a user is a collaborator of a repository
func IsCollaborator(repoID, userID int64) (bool, error) {
	return isCollaborator(db.GetEngine(db.DefaultContext), repoID, userID)
}

func changeCollaborationAccessMode(e db.Engine, repo *repo_model.Repository, uid int64, mode perm.AccessMode) error {
	// Discard invalid input
	if mode <= perm.AccessModeNone || mode > perm.AccessModeOwner {
		return nil
	}

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
func ChangeCollaborationAccessMode(repo *repo_model.Repository, uid int64, mode perm.AccessMode) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := changeCollaborationAccessMode(db.GetEngine(ctx), repo, uid, mode); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteCollaboration removes collaboration relation between the user and repository.
func DeleteCollaboration(repo *repo_model.Repository, uid int64) (err error) {
	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: uid,
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if has, err := db.GetEngine(ctx).Delete(collaboration); err != nil || has == 0 {
		return err
	} else if err = recalculateAccesses(ctx, repo); err != nil {
		return err
	}

	if err = repo_model.WatchRepoCtx(ctx, uid, repo.ID, false); err != nil {
		return err
	}

	if err = reconsiderWatches(ctx, repo, uid); err != nil {
		return err
	}

	// Unassign a user from any issue (s)he has been assigned to in the repository
	if err := reconsiderRepoIssuesAssignee(ctx, repo, uid); err != nil {
		return err
	}

	return committer.Commit()
}

func reconsiderRepoIssuesAssignee(ctx context.Context, repo *repo_model.Repository, uid int64) error {
	user, err := user_model.GetUserByIDEngine(db.GetEngine(ctx), uid)
	if err != nil {
		return err
	}

	if canAssigned, err := canBeAssigned(ctx, user, repo, true); err != nil || canAssigned {
		return err
	}

	if _, err := db.GetEngine(ctx).Where(builder.Eq{"assignee_id": uid}).
		In("issue_id", builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repo.ID})).
		Delete(&IssueAssignees{}); err != nil {
		return fmt.Errorf("Could not delete assignee[%d] %v", uid, err)
	}
	return nil
}

func reconsiderWatches(ctx context.Context, repo *repo_model.Repository, uid int64) error {
	if has, err := hasAccess(ctx, uid, repo); err != nil || has {
		return err
	}
	if err := repo_model.WatchRepoCtx(ctx, uid, repo.ID, false); err != nil {
		return err
	}

	// Remove all IssueWatches a user has subscribed to in the repository
	return removeIssueWatchersByRepoID(db.GetEngine(ctx), uid, repo.ID)
}

func getRepoTeams(e db.Engine, repo *repo_model.Repository) (teams []*Team, err error) {
	return teams, e.
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		Where("team.org_id = ?", repo.OwnerID).
		And("team_repo.repo_id=?", repo.ID).
		OrderBy("CASE WHEN name LIKE '" + ownerTeamName + "' THEN '' ELSE name END").
		Find(&teams)
}

// GetRepoTeams gets the list of teams that has access to the repository
func GetRepoTeams(repo *repo_model.Repository) ([]*Team, error) {
	return getRepoTeams(db.GetEngine(db.DefaultContext), repo)
}

// IsOwnerMemberCollaborator checks if a provided user is the owner, a collaborator or a member of a team in a repository
func IsOwnerMemberCollaborator(repo *repo_model.Repository, userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	teamMember, err := db.GetEngine(db.DefaultContext).Join("INNER", "team_repo", "team_repo.team_id = team_user.team_id").
		Join("INNER", "team_unit", "team_unit.team_id = team_user.team_id").
		Where("team_repo.repo_id = ?", repo.ID).
		And("team_unit.`type` = ?", unit.TypeCode).
		And("team_user.uid = ?", userID).Table("team_user").Exist(&TeamUser{})
	if err != nil {
		return false, err
	}
	if teamMember {
		return true, nil
	}

	return db.GetEngine(db.DefaultContext).Get(&Collaboration{RepoID: repo.ID, UserID: userID})
}
