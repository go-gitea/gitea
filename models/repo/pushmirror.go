// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrPushMirrorNotExist mirror does not exist error
var ErrPushMirrorNotExist = util.NewNotExistErrorf("PushMirror does not exist")

// PushMirror represents mirror information of a repository.
type PushMirror struct {
	ID            int64       `xorm:"pk autoincr"`
	RepoID        int64       `xorm:"INDEX"`
	Repo          *Repository `xorm:"-"`
	RemoteName    string
	RemoteAddress string `xorm:"VARCHAR(2048)"`

	SyncOnCommit   bool `xorm:"NOT NULL DEFAULT true"`
	Interval       time.Duration
	CreatedUnix    timeutil.TimeStamp `xorm:"created"`
	LastUpdateUnix timeutil.TimeStamp `xorm:"INDEX last_update"`
	LastError      string             `xorm:"text"`
}

type PushMirrorOptions struct {
	ID         int64
	RepoID     int64
	RemoteName string
}

func (opts *PushMirrorOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.RemoteName != "" {
		cond = cond.And(builder.Eq{"remote_name": opts.RemoteName})
	}
	if opts.ID > 0 {
		cond = cond.And(builder.Eq{"id": opts.ID})
	}
	return cond
}

func init() {
	db.RegisterModel(new(PushMirror))
}

// GetRepository returns the path of the repository.
func (m *PushMirror) GetRepository(ctx context.Context) *Repository {
	if m.Repo != nil {
		return m.Repo
	}
	var err error
	m.Repo, err = GetRepositoryByID(ctx, m.RepoID)
	if err != nil {
		log.Error("getRepositoryByID[%d]: %v", m.ID, err)
	}
	return m.Repo
}

// GetRemoteName returns the name of the remote.
func (m *PushMirror) GetRemoteName() string {
	return m.RemoteName
}

// InsertPushMirror inserts a push-mirror to database
func InsertPushMirror(ctx context.Context, m *PushMirror) error {
	_, err := db.GetEngine(ctx).Insert(m)
	return err
}

// UpdatePushMirror updates the push-mirror
func UpdatePushMirror(ctx context.Context, m *PushMirror) error {
	_, err := db.GetEngine(ctx).ID(m.ID).AllCols().Update(m)
	return err
}

// UpdatePushMirrorInterval updates the push-mirror
func UpdatePushMirrorInterval(ctx context.Context, m *PushMirror) error {
	_, err := db.GetEngine(ctx).ID(m.ID).Cols("interval").Update(m)
	return err
}

func DeletePushMirrors(ctx context.Context, opts PushMirrorOptions) error {
	if opts.RepoID > 0 {
		_, err := db.GetEngine(ctx).Where(opts.toConds()).Delete(&PushMirror{})
		return err
	}
	return util.NewInvalidArgumentErrorf("repoID required and must be set")
}

func GetPushMirror(ctx context.Context, opts PushMirrorOptions) (*PushMirror, error) {
	mirror := &PushMirror{}
	exist, err := db.GetEngine(ctx).Where(opts.toConds()).Get(mirror)
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrPushMirrorNotExist
	}
	return mirror, nil
}

// GetPushMirrorsByRepoID returns push-mirror information of a repository.
func GetPushMirrorsByRepoID(ctx context.Context, repoID int64, listOptions db.ListOptions) ([]*PushMirror, int64, error) {
	sess := db.GetEngine(ctx).Where("repo_id = ?", repoID)
	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
		mirrors := make([]*PushMirror, 0, listOptions.PageSize)
		count, err := sess.FindAndCount(&mirrors)
		return mirrors, count, err
	}
	mirrors := make([]*PushMirror, 0, 10)
	count, err := sess.FindAndCount(&mirrors)
	return mirrors, count, err
}

// GetPushMirrorsSyncedOnCommit returns push-mirrors for this repo that should be updated by new commits
func GetPushMirrorsSyncedOnCommit(ctx context.Context, repoID int64) ([]*PushMirror, error) {
	mirrors := make([]*PushMirror, 0, 10)
	return mirrors, db.GetEngine(ctx).
		Where("repo_id = ? AND sync_on_commit = ?", repoID, true).
		Find(&mirrors)
}

// PushMirrorsIterate iterates all push-mirror repositories.
func PushMirrorsIterate(ctx context.Context, limit int, f func(idx int, bean any) error) error {
	sess := db.GetEngine(ctx).
		Where("last_update + (`interval` / ?) <= ?", time.Second, time.Now().Unix()).
		And("`interval` != 0").
		OrderBy("last_update ASC")
	if limit > 0 {
		sess = sess.Limit(limit)
	}
	return sess.Iterate(new(PushMirror), f)
}
