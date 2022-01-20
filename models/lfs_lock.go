// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
)

// LFSLock represents a git lfs lock of repository.
type LFSLock struct {
	ID      int64     `xorm:"pk autoincr"`
	RepoID  int64     `xorm:"INDEX NOT NULL"`
	OwnerID int64     `xorm:"INDEX NOT NULL"`
	Path    string    `xorm:"TEXT"`
	Created time.Time `xorm:"created"`
}

func init() {
	db.RegisterModel(new(LFSLock))
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (l *LFSLock) BeforeInsert() {
	l.Path = cleanPath(l.Path)
}

func cleanPath(p string) string {
	return path.Clean("/" + p)[1:]
}

// CreateLFSLock creates a new lock.
func CreateLFSLock(repo *repo_model.Repository, lock *LFSLock) (*LFSLock, error) {
	err := CheckLFSAccessForRepo(lock.OwnerID, repo, perm.AccessModeWrite)
	if err != nil {
		return nil, err
	}

	lock.Path = cleanPath(lock.Path)
	lock.RepoID = repo.ID

	l, err := GetLFSLock(repo, lock.Path)
	if err == nil {
		return l, ErrLFSLockAlreadyExist{lock.RepoID, lock.Path}
	}
	if !IsErrLFSLockNotExist(err) {
		return nil, err
	}

	err = db.Insert(db.DefaultContext, lock)
	return lock, err
}

// GetLFSLock returns release by given path.
func GetLFSLock(repo *repo_model.Repository, path string) (*LFSLock, error) {
	path = cleanPath(path)
	rel := &LFSLock{RepoID: repo.ID}
	has, err := db.GetEngine(db.DefaultContext).Where("lower(path) = ?", strings.ToLower(path)).Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrLFSLockNotExist{0, repo.ID, path}
	}
	return rel, nil
}

// GetLFSLockByID returns release by given id.
func GetLFSLockByID(id int64) (*LFSLock, error) {
	lock := new(LFSLock)
	has, err := db.GetEngine(db.DefaultContext).ID(id).Get(lock)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSLockNotExist{id, 0, ""}
	}
	return lock, nil
}

// GetLFSLockByRepoID returns a list of locks of repository.
func GetLFSLockByRepoID(repoID int64, page, pageSize int) ([]*LFSLock, error) {
	e := db.GetEngine(db.DefaultContext)
	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		e.Limit(pageSize, start)
	}
	lfsLocks := make([]*LFSLock, 0, pageSize)
	return lfsLocks, e.Find(&lfsLocks, &LFSLock{RepoID: repoID})
}

// GetTreePathLock returns LSF lock for the treePath
func GetTreePathLock(repoID int64, treePath string) (*LFSLock, error) {
	if !setting.LFS.StartServer {
		return nil, nil
	}

	locks, err := GetLFSLockByRepoID(repoID, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, lock := range locks {
		if lock.Path == treePath {
			return lock, nil
		}
	}
	return nil, nil
}

// CountLFSLockByRepoID returns a count of all LFSLocks associated with a repository.
func CountLFSLockByRepoID(repoID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Count(&LFSLock{RepoID: repoID})
}

// DeleteLFSLockByID deletes a lock by given ID.
func DeleteLFSLockByID(id int64, repo *repo_model.Repository, u *user_model.User, force bool) (*LFSLock, error) {
	lock, err := GetLFSLockByID(id)
	if err != nil {
		return nil, err
	}

	err = CheckLFSAccessForRepo(u.ID, repo, perm.AccessModeWrite)
	if err != nil {
		return nil, err
	}

	if !force && u.ID != lock.OwnerID {
		return nil, fmt.Errorf("user doesn't own lock and force flag is not set")
	}

	_, err = db.GetEngine(db.DefaultContext).ID(id).Delete(new(LFSLock))
	return lock, err
}

// CheckLFSAccessForRepo check needed access mode base on action
func CheckLFSAccessForRepo(ownerID int64, repo *repo_model.Repository, mode perm.AccessMode) error {
	if ownerID == 0 {
		return ErrLFSUnauthorizedAction{repo.ID, "undefined", mode}
	}
	u, err := user_model.GetUserByID(ownerID)
	if err != nil {
		return err
	}
	perm, err := GetUserRepoPermission(repo, u)
	if err != nil {
		return err
	}
	if !perm.CanAccess(mode, unit.TypeCode) {
		return ErrLFSUnauthorizedAction{repo.ID, u.DisplayName(), mode}
	}
	return nil
}
