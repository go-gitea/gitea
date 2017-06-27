// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
)

// Collaboration represent the relation between an individual and a repository.
type Collaboration struct {
	ID     int64      `xorm:"pk autoincr"`
	RepoID int64      `xorm:"UNIQUE(s) INDEX NOT NULL"`
	UserID int64      `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Mode   AccessMode `xorm:"DEFAULT 2 NOT NULL"`
}

// ModeI18nKey returns the collaboration mode I18n Key
func (c *Collaboration) ModeI18nKey() string {
	switch c.Mode {
	case AccessModeRead:
		return "repo.settings.collaboration.read"
	case AccessModeWrite:
		return "repo.settings.collaboration.write"
	case AccessModeAdmin:
		return "repo.settings.collaboration.admin"
	default:
		return "repo.settings.collaboration.undefined"
	}
}

// AddCollaborator adds new collaboration to a repository with default access mode.
func (repo *Repository) AddCollaborator(u *User) error {
	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: u.ID,
	}

	has, err := x.Get(collaboration)
	if err != nil {
		return err
	} else if has {
		return nil
	}
	collaboration.Mode = AccessModeWrite

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.InsertOne(collaboration); err != nil {
		return err
	}

	if repo.Owner.IsOrganization() {
		err = repo.recalculateTeamAccesses(sess, 0)
	} else {
		err = repo.recalculateAccesses(sess)
	}
	if err != nil {
		return fmt.Errorf("recalculateAccesses 'team=%v': %v", repo.Owner.IsOrganization(), err)
	}

	return sess.Commit()
}

func (repo *Repository) getCollaborations(e Engine) ([]*Collaboration, error) {
	var collaborations []*Collaboration
	return collaborations, e.Find(&collaborations, &Collaboration{RepoID: repo.ID})
}

// Collaborator represents a user with collaboration details.
type Collaborator struct {
	*User
	Collaboration *Collaboration
}

func (repo *Repository) getCollaborators(e Engine) ([]*Collaborator, error) {
	collaborations, err := repo.getCollaborations(e)
	if err != nil {
		return nil, fmt.Errorf("getCollaborations: %v", err)
	}

	collaborators := make([]*Collaborator, len(collaborations))
	for i, c := range collaborations {
		user, err := getUserByID(e, c.UserID)
		if err != nil {
			return nil, err
		}
		collaborators[i] = &Collaborator{
			User:          user,
			Collaboration: c,
		}
	}
	return collaborators, nil
}

// GetCollaborators returns the collaborators for a repository
func (repo *Repository) GetCollaborators() ([]*Collaborator, error) {
	return repo.getCollaborators(x)
}

func (repo *Repository) isCollaborator(e Engine, userID int64) (bool, error) {
	return e.Get(&Collaboration{RepoID: repo.ID, UserID: userID})
}

// IsCollaborator check if a user is a collaborator of a repository
func (repo *Repository) IsCollaborator(userID int64) (bool, error) {
	return repo.isCollaborator(x, userID)
}

// ChangeCollaborationAccessMode sets new access mode for the collaboration.
func (repo *Repository) ChangeCollaborationAccessMode(uid int64, mode AccessMode) error {
	// Discard invalid input
	if mode <= AccessModeNone || mode > AccessModeOwner {
		return nil
	}

	collaboration := &Collaboration{
		RepoID: repo.ID,
		UserID: uid,
	}
	has, err := x.Get(collaboration)
	if err != nil {
		return fmt.Errorf("get collaboration: %v", err)
	} else if !has {
		return nil
	}

	if collaboration.Mode == mode {
		return nil
	}
	collaboration.Mode = mode

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.
		Id(collaboration.ID).
		AllCols().
		Update(collaboration); err != nil {
		return fmt.Errorf("update collaboration: %v", err)
	} else if _, err = sess.Exec("UPDATE access SET mode = ? WHERE user_id = ? AND repo_id = ?", mode, uid, repo.ID); err != nil {
		return fmt.Errorf("update access table: %v", err)
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

	return sess.Commit()
}
