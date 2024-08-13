// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// LFSLock represents a git lfs lock of repository.
type LFSLock struct {
	ID      int64            `xorm:"pk autoincr"`
	RepoID  int64            `xorm:"INDEX NOT NULL"`
	OwnerID int64            `xorm:"INDEX NOT NULL"`
	Owner   *user_model.User `xorm:"-"`
	Path    string           `xorm:"TEXT"`
	Created time.Time        `xorm:"created"`
}

func init() {
	db.RegisterModel(new(LFSLock))
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (l *LFSLock) BeforeInsert() {
	l.Path = util.PathJoinRel(l.Path)
}

// LoadAttributes loads attributes of the lock.
func (l *LFSLock) LoadAttributes(ctx context.Context) error {
	// Load owner
	if err := l.LoadOwner(ctx); err != nil {
		return fmt.Errorf("load owner: %w", err)
	}

	return nil
}

// LoadOwner loads owner of the lock.
func (l *LFSLock) LoadOwner(ctx context.Context) error {
	if l.Owner != nil {
		return nil
	}

	owner, err := user_model.GetUserByID(ctx, l.OwnerID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			l.Owner = user_model.NewGhostUser()
			return nil
		}
		return err
	}
	l.Owner = owner

	return nil
}

// CreateLFSLock creates a new lock.
func CreateLFSLock(ctx context.Context, repo *repo_model.Repository, lock *LFSLock) (*LFSLock, error) {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	if err := CheckLFSAccessForRepo(dbCtx, lock.OwnerID, repo, perm.AccessModeWrite); err != nil {
		return nil, err
	}

	lock.Path = util.PathJoinRel(lock.Path)
	lock.RepoID = repo.ID

	l, err := GetLFSLock(dbCtx, repo, lock.Path)
	if err == nil {
		return l, ErrLFSLockAlreadyExist{lock.RepoID, lock.Path}
	}
	if !IsErrLFSLockNotExist(err) {
		return nil, err
	}

	if err := db.Insert(dbCtx, lock); err != nil {
		return nil, err
	}

	return lock, committer.Commit()
}

// GetLFSLock returns release by given path.
func GetLFSLock(ctx context.Context, repo *repo_model.Repository, path string) (*LFSLock, error) {
	path = util.PathJoinRel(path)
	rel := &LFSLock{RepoID: repo.ID}
	has, err := db.GetEngine(ctx).Where("lower(path) = ?", strings.ToLower(path)).Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrLFSLockNotExist{0, repo.ID, path}
	}
	return rel, nil
}

// GetLFSLockByID returns release by given id.
func GetLFSLockByID(ctx context.Context, id int64) (*LFSLock, error) {
	lock := new(LFSLock)
	has, err := db.GetEngine(ctx).ID(id).Get(lock)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSLockNotExist{id, 0, ""}
	}
	return lock, nil
}

// GetLFSLockByRepoID returns a list of locks of repository.
func GetLFSLockByRepoID(ctx context.Context, repoID int64, page, pageSize int) (LFSLockList, error) {
	e := db.GetEngine(ctx)
	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		e.Limit(pageSize, start)
	}
	lfsLocks := make(LFSLockList, 0, pageSize)
	return lfsLocks, e.Find(&lfsLocks, &LFSLock{RepoID: repoID})
}

// GetTreePathLock returns LSF lock for the treePath
func GetTreePathLock(ctx context.Context, repoID int64, treePath string) (*LFSLock, error) {
	if !setting.LFS.StartServer {
		return nil, nil
	}

	locks, err := GetLFSLockByRepoID(ctx, repoID, 0, 0)
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
func CountLFSLockByRepoID(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Count(&LFSLock{RepoID: repoID})
}

// DeleteLFSLockByID deletes a lock by given ID.
func DeleteLFSLockByID(ctx context.Context, id int64, repo *repo_model.Repository, u *user_model.User, force bool) (*LFSLock, error) {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	lock, err := GetLFSLockByID(dbCtx, id)
	if err != nil {
		return nil, err
	}

	if err := CheckLFSAccessForRepo(dbCtx, u.ID, repo, perm.AccessModeWrite); err != nil {
		return nil, err
	}

	if !force && u.ID != lock.OwnerID {
		return nil, errors.New("user doesn't own lock and force flag is not set")
	}

	if _, err := db.GetEngine(dbCtx).ID(id).Delete(new(LFSLock)); err != nil {
		return nil, err
	}

	return lock, committer.Commit()
}

// CheckLFSAccessForRepo check needed access mode base on action
func CheckLFSAccessForRepo(ctx context.Context, ownerID int64, repo *repo_model.Repository, mode perm.AccessMode) error {
	if ownerID == 0 {
		return ErrLFSUnauthorizedAction{repo.ID, "undefined", mode}
	}
	u, err := user_model.GetUserByID(ctx, ownerID)
	if err != nil {
		return err
	}
	perm, err := access_model.GetUserRepoPermission(ctx, repo, u)
	if err != nil {
		return err
	}
	if !perm.CanAccess(mode, unit.TypeCode) {
		return ErrLFSUnauthorizedAction{repo.ID, u.DisplayName(), mode}
	}
	return nil
}
