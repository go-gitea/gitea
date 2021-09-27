// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Collaboration represent the relation between an individual and a repository.
type Collaboration struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	UserID      int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Mode        AccessMode         `xorm:"DEFAULT 2 NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (repo *Repository) addCollaborator(e Engine, u *User) error {
	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: u.ID,
	}

	has, err := e.Get(collaboration)
	if err != nil {
		return err
	} else if has {
		return nil
	}
	collaboration.Mode = AccessModeWrite

	if _, err = e.InsertOne(collaboration); err != nil {
		return err
	}

	return repo.recalculateUserAccess(e, u.ID)
}

// AddCollaborator adds new collaboration to a repository with default access mode.
func (repo *Repository) AddCollaborator(u *User) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := repo.addCollaborator(sess, u); err != nil {
		return err
	}

	return sess.Commit()
}

func (repo *Repository) getCollaborations(e Engine, listOptions ListOptions) ([]*Collaboration, error) {
	if listOptions.Page == 0 {
		collaborations := make([]*Collaboration, 0, 8)
		return collaborations, e.Find(&collaborations, &Collaboration{RepoID: repo.ID})
	}

	e = listOptions.setEnginePagination(e)

	collaborations := make([]*Collaboration, 0, listOptions.PageSize)
	return collaborations, e.Find(&collaborations, &Collaboration{RepoID: repo.ID})
}

// Collaborator represents a user with collaboration details.
type Collaborator struct {
	*User
	Collaboration *Collaboration
}

func (repo *Repository) getCollaborators(e Engine, listOptions ListOptions) ([]*Collaborator, error) {
	collaborations, err := repo.getCollaborations(e, listOptions)
	if err != nil {
		return nil, fmt.Errorf("getCollaborations: %v", err)
	}

	collaborators := make([]*Collaborator, 0, len(collaborations))
	for _, c := range collaborations {
		user, err := getUserByID(e, c.UserID)
		if err != nil {
			if IsErrUserNotExist(err) {
				log.Warn("Inconsistent DB: User: %d is listed as collaborator of %-v but does not exist", c.UserID, repo)
				user = NewGhostUser()
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
func (repo *Repository) GetCollaborators(listOptions ListOptions) ([]*Collaborator, error) {
	return repo.getCollaborators(x, listOptions)
}

func (repo *Repository) getCollaboration(e Engine, uid int64) (*Collaboration, error) {
	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: uid,
	}
	has, err := e.Get(collaboration)
	if !has {
		collaboration = nil
	}
	return collaboration, err
}

func (repo *Repository) isCollaborator(e Engine, userID int64) (bool, error) {
	return e.Get(&Collaboration{RepoID: repo.ID, UserID: userID})
}

// IsCollaborator check if a user is a collaborator of a repository
func (repo *Repository) IsCollaborator(userID int64) (bool, error) {
	return repo.isCollaborator(x, userID)
}

func (repo *Repository) changeCollaborationAccessMode(e Engine, uid int64, mode AccessMode) error {
	// Discard invalid input
	if mode <= AccessModeNone || mode > AccessModeOwner {
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
func (repo *Repository) ChangeCollaborationAccessMode(uid int64, mode AccessMode) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := repo.changeCollaborationAccessMode(sess, uid, mode); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteCollaboration removes collaboration relation between the user and repository.
func (repo *Repository) DeleteCollaboration(uid int64) (err error) {
	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: uid,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if has, err := sess.Delete(collaboration); err != nil || has == 0 {
		return err
	} else if err = repo.recalculateAccesses(sess); err != nil {
		return err
	}

	if err = watchRepo(sess, uid, repo.ID, false); err != nil {
		return err
	}

	if err = repo.reconsiderWatches(sess, uid); err != nil {
		return err
	}

	// Unassign a user from any issue (s)he has been assigned to in the repository
	if err := repo.reconsiderIssueAssignees(sess, uid); err != nil {
		return err
	}

	return sess.Commit()
}

func (repo *Repository) reconsiderIssueAssignees(e Engine, uid int64) error {
	user, err := getUserByID(e, uid)
	if err != nil {
		return err
	}

	if canAssigned, err := canBeAssigned(e, user, repo, true); err != nil || canAssigned {
		return err
	}

	if _, err := e.Where(builder.Eq{"assignee_id": uid}).
		In("issue_id", builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repo.ID})).
		Delete(&IssueAssignees{}); err != nil {
		return fmt.Errorf("Could not delete assignee[%d] %v", uid, err)
	}
	return nil
}

func (repo *Repository) reconsiderWatches(e Engine, uid int64) error {
	if has, err := hasAccess(e, uid, repo); err != nil || has {
		return err
	}

	if err := watchRepo(e, uid, repo.ID, false); err != nil {
		return err
	}

	// Remove all IssueWatches a user has subscribed to in the repository
	return removeIssueWatchersByRepoID(e, uid, repo.ID)
}

func (repo *Repository) getRepoTeams(e Engine) (teams []*Team, err error) {
	return teams, e.
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		Where("team.org_id = ?", repo.OwnerID).
		And("team_repo.repo_id=?", repo.ID).
		OrderBy("CASE WHEN name LIKE '" + ownerTeamName + "' THEN '' ELSE name END").
		Find(&teams)
}

// GetRepoTeams gets the list of teams that has access to the repository
func (repo *Repository) GetRepoTeams() ([]*Team, error) {
	return repo.getRepoTeams(x)
}

// IsOwnerMemberCollaborator checks if a provided user is the owner, a collaborator or a member of a team in a repository
func (repo *Repository) IsOwnerMemberCollaborator(userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	teamMember, err := x.Join("INNER", "team_repo", "team_repo.team_id = team_user.team_id").
		Join("INNER", "team_unit", "team_unit.team_id = team_user.team_id").
		Where("team_repo.repo_id = ?", repo.ID).
		And("team_unit.`type` = ?", UnitTypeCode).
		And("team_user.uid = ?", userID).Table("team_user").Exist(&TeamUser{})
	if err != nil {
		return false, err
	}
	if teamMember {
		return true, nil
	}

	return x.Get(&Collaboration{RepoID: repo.ID, UserID: userID})
}
