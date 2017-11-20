// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	api "code.gitea.io/sdk/gitea"
	"fmt"
	"strconv"
	"time"
)

// LFSLock represents a git lfs lfock of repository.
type LFSLock struct {
	ID          int64       `xorm:"pk autoincr"`
	RepoID      int64       `xorm:"INDEX UNIQUE(n)"`
	Repo        *Repository `xorm:"-"`
	OwnerID     int64       `xorm:"INDEX"`
	Owner       *User       `xorm:"-"`
	Path        string
	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (l *LFSLock) BeforeInsert() {
	if l.CreatedUnix == 0 {
		l.CreatedUnix = time.Now().Unix()
	}
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (l *LFSLock) AfterLoad() {
	l.Created = time.Unix(l.CreatedUnix, 0).Local()
}

func (l *LFSLock) loadAttributes(e Engine) error {
	var err error
	if l.Repo == nil {
		l.Repo, err = GetRepositoryByID(l.RepoID)
		if err != nil {
			return err
		}
	}
	if l.Owner == nil {
		l.Owner, err = GetUserByID(l.OwnerID)
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadAttributes load repo and publisher attributes for a lock
func (l *LFSLock) LoadAttributes() error {
	return l.loadAttributes(x)
}

// APIFormat convert a Release to lfs.LFSLock
func (l *LFSLock) APIFormat() *api.LFSLock {
	//TODO move to api
	return &api.LFSLock{
		ID:       strconv.FormatInt(l.ID, 10),
		Path:     l.Path,
		LockedAt: l.Created,
		Owner: &api.LFSLockOwner{
			Name: l.Owner.DisplayName(),
		},
	}
}

// IsLFSLockExist returns true if lock with given path already exists.
func IsLFSLockExist(repoID int64, path string) (bool, error) {
	return x.Get(&LFSLock{RepoID: repoID, Path: path}) //TODO Define if path should needed to be lower for windows compat ?
}

// CreateLFSLock creates a new lock.
func CreateLFSLock(lock *LFSLock, u *User) (*LFSLock, error) {
	isExist, err := IsLFSLockExist(lock.RepoID, lock.Path)
	if err != nil {
		return nil, err
	} else if isExist {
		l, err := GetLFSLock(lock.RepoID, lock.Path)
		if err != nil {
			return nil, err
		}
		return l, ErrLFSLockAlreadyExist{lock.RepoID, lock.Path}
	}

	repo, err := GetRepositoryByID(lock.RepoID)
	if err != nil {
		return nil, err
	}

	has, err := HasAccess(u.ID, repo, AccessModeWrite)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSLockUnauthorizedAction{0, lock.RepoID, u, "create"}
	}

	_, err = x.InsertOne(lock)
	return lock, err
}

// GetLFSLock returns release by given path.
func GetLFSLock(repoID int64, path string) (*LFSLock, error) {
	isExist, err := IsLFSLockExist(repoID, path)
	if err != nil {
		return nil, err
	} else if !isExist {
		return nil, ErrLFSLockNotExist{0, repoID, path}
	}

	rel := &LFSLock{RepoID: repoID, Path: path} //TODO Define if path should needed to be lower for windows compat ?
	_, err = x.Get(rel)
	return rel, err
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
	return locks, err
}

// DeleteLFSLockByID deletes a lock by given ID.
func DeleteLFSLockByID(id int64, u *User, force bool) error {

	lock, err := GetLFSLockByID(id)
	if err != nil {
		return err
	}
	repo, err := GetRepositoryByID(lock.RepoID)
	if err != nil {
		return err
	}

	has, err := HasAccess(u.ID, repo, AccessModeWrite)
	if err != nil {
		return err
	} else if !has {
		return ErrLFSLockUnauthorizedAction{id, repo.ID, u, "delete"}
	}

	if !force && u.ID != lock.OwnerID {
		return fmt.Errorf("user doesn't own lock and force flag is not set")
	}

	_, err = x.ID(id).Delete(new(LFSLock))
	return err
}
