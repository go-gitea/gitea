// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	api "code.gitea.io/sdk/gitea"
)

// LFSLock represents a git lfs lock of repository.
type LFSLock struct {
	ID      int64     `xorm:"pk autoincr"`
	RepoID  int64     `xorm:"INDEX NOT NULL"`
	Owner   *User     `xorm:"-"`
	OwnerID int64     `xorm:"INDEX NOT NULL"`
	Path    string    `xorm:"TEXT"`
	Created time.Time `xorm:"created"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (l *LFSLock) BeforeInsert() {
	l.OwnerID = l.Owner.ID
	l.Path = cleanPath(l.Path)
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (l *LFSLock) AfterLoad() {
	l.Owner, _ = GetUserByID(l.OwnerID)
}

func cleanPath(p string) string {
	return strings.ToLower(path.Clean(p))
}

// APIFormat convert a Release to lfs.LFSLock
func (l *LFSLock) APIFormat() *api.LFSLock {
	return &api.LFSLock{
		ID:       strconv.FormatInt(l.ID, 10),
		Path:     l.Path,
		LockedAt: l.Created,
		Owner: &api.LFSLockOwner{
			Name: l.Owner.DisplayName(),
		},
	}
}

// CreateLFSLock creates a new lock.
func CreateLFSLock(lock *LFSLock) (*LFSLock, error) {
	err := CheckLFSAccessForRepo(lock.Owner, lock.RepoID, "create")
	if err != nil {
		return nil, err
	}

	l, err := GetLFSLock(lock.RepoID, lock.Path)
	if err == nil {
		return l, ErrLFSLockAlreadyExist{lock.RepoID, lock.Path}
	}
	if !IsErrLFSLockNotExist(err) {
		return nil, err
	}

	_, err = x.InsertOne(lock)
	return lock, err
}

// GetLFSLock returns release by given path.
func GetLFSLock(repoID int64, path string) (*LFSLock, error) {
	path = cleanPath(path)
	rel := &LFSLock{RepoID: repoID, Path: path}
	has, err := x.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrLFSLockNotExist{0, repoID, path}
	}
	return rel, nil
}

// GetLFSLockByID returns release by given id.
func GetLFSLockByID(id int64) (*LFSLock, error) {
	lock := new(LFSLock)
	has, err := x.ID(id).Get(lock)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSLockNotExist{id, 0, ""}
	}
	return lock, nil
}

// GetLFSLockByRepoID returns a list of locks of repository.
func GetLFSLockByRepoID(repoID int64) (locks []*LFSLock, err error) {
	err = x.Where("repo_id = ?", repoID).Find(&locks)
	return
}

// DeleteLFSLockByID deletes a lock by given ID.
func DeleteLFSLockByID(id int64, u *User, force bool) (*LFSLock, error) {
	lock, err := GetLFSLockByID(id)
	if err != nil {
		return nil, err
	}

	err = CheckLFSAccessForRepo(u, lock.RepoID, "delete")
	if err != nil {
		return nil, err
	}

	if !force && u.ID != lock.OwnerID {
		return nil, fmt.Errorf("user doesn't own lock and force flag is not set")
	}

	_, err = x.ID(id).Delete(new(LFSLock))
	return lock, err
}

//CheckLFSAccessForRepo check needed access mode base on action
func CheckLFSAccessForRepo(u *User, repoID int64, action string) error {
	if u == nil {
		return ErrLFSLockUnauthorizedAction{repoID, "undefined", action}
	}
	mode := AccessModeRead
	if action == "create" || action == "delete" || action == "verify" {
		mode = AccessModeWrite
	}

	repo, err := GetRepositoryByID(repoID)
	if err != nil {
		return err
	}
	has, err := HasAccess(u.ID, repo, mode)
	if err != nil {
		return err
	} else if !has {
		return ErrLFSLockUnauthorizedAction{repo.ID, u.DisplayName(), action}
	}
	return nil
}
