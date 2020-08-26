// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
)

// UserPinnedRepo represents a pinned repo by an user or org.
type UserPinnedRepo struct {
	ID      int64 `xorm:"pk autoincr"`
	UID     int64 `xorm:"INDEX NOT NULL"`
	RepoID  int64 `xorm:"NOT NULL"`
	IsOwned bool  `xorm:"NOT NULL"`
}

// AddPinnedRepo add a pinned repo
func (u *User) AddPinnedRepo(repo *Repository) (err error) {
	exist := false
	if exist, err = u.IsPinnedRepoExist(repo.ID); err != nil {
		return
	}

	if exist {
		return ErrUserPinnedRepoAlreadyExist{UID: u.ID, RepoID: repo.ID}
	}

	r := &UserPinnedRepo{
		UID:     u.ID,
		RepoID:  repo.ID,
		IsOwned: u.ID == repo.OwnerID,
	}

	_, err = x.Insert(r)
	return
}

// RemovePinnedRepo remove a pinned repo
func (u *User) RemovePinnedRepo(repoID int64) (err error) {
	exist := false
	if exist, err = u.IsPinnedRepoExist(repoID); err != nil {
		return
	}

	if !exist {
		return ErrUserPinnedRepoNotExist{UID: u.ID, RepoID: repoID}
	}

	_, err = x.Delete(&UserPinnedRepo{UID: u.ID, RepoID: repoID})
	return
}

// IsPinnedRepoExist check if this repo is pinned
func (u *User) IsPinnedRepoExist(repoID int64) (isExist bool, err error) {
	return x.Exist(&UserPinnedRepo{UID: u.ID, RepoID: repoID})
}

// GetPinnedRepos get pinned repos
func (u *User) GetPinnedRepos(actor *User, onlyIncludeNotOwned, loadAttributes bool) (repos RepositoryList, err error) {
	var cond = builder.NewCond()
	repos = make(RepositoryList, 0, 10)

	if actor == nil {
		if u.IsOrganization() && u.Visibility != structs.VisibleTypePublic {
			return
		}
		cond = cond.And(builder.Eq{"is_private": false})
	} else if actor.ID != u.ID && !actor.IsAdmin {
		// OK we're in the context of a User
		cond = cond.And(accessibleRepositoryCondition(actor))
	}

	var idBuilder *builder.Builder

	if onlyIncludeNotOwned {
		idBuilder = builder.Select("repo_id").
			From("user_pinned_repo").
			Where(builder.Eq{"uid": u.ID, "is_owned": false})
	} else {
		idBuilder = builder.Select("repo_id").
			From("user_pinned_repo").
			Where(builder.Eq{"uid": u.ID})
	}

	cond = cond.And(builder.In("id", idBuilder))
	if err = x.Where(cond).Find(&repos); err != nil {
		return nil, err
	}

	if loadAttributes {
		if err = repos.LoadAttributes(); err != nil {
			return nil, err
		}
	}

	return
}

// ChangePinnedRepoIsOwndStatus change isPinned repo status for a repo
func ChangePinnedRepoIsOwndStatus(repoID, oldOwnerID, newOwnerID int64) (err error) {
	_, err = x.Exec("UPDATE `user_pinned_repo` SET is_owned=? WHERE uid=? AND repo_id=?", false, oldOwnerID, repoID)
	if err != nil {
		return
	}

	_, err = x.Exec("UPDATE `user_pinned_repo` SET is_owned=? WHERE uid=? AND repo_id=?", true, newOwnerID, repoID)
	return
}
