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
	UID    int64 `xorm:"pk INDEX NOT NULL"`
	RepoID int64 `xorm:"pk NOT NULL"`
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
		UID:    u.ID,
		RepoID: repo.ID,
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

// GetPinnedRepoIDs get repos id
func (u *User) GetPinnedRepoIDs(actor *User) (results []int64, err error) {
	var cond = builder.NewCond()
	results = make([]int64, 0, 10)

	if actor == nil {
		if u.IsOrganization() && u.Visibility != structs.VisibleTypePublic {
			return
		}
		cond = cond.And(builder.Eq{"is_private": false})
	} else if actor.ID != u.ID && !actor.IsAdmin {
		// OK we're in the context of a User
		cond = cond.And(accessibleRepositoryCondition(actor))
	}

	idBuilder := builder.Select("repo_id").
		From("user_pinned_repo").
		Where(builder.Eq{"uid": u.ID})

	cond = cond.And(builder.In("id", idBuilder))

	err = x.Table("repository").Cols("id").Where(cond).Find(&results)
	return
}
