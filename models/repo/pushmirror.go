// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

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
	db.ListOptions
	ID         int64
	RepoID     int64
	RemoteName string
}

func (opts PushMirrorOptions) ToConds() builder.Cond {
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
		_, err := db.Delete[PushMirror](ctx, opts)
		return err
	}
	return util.NewInvalidArgumentErrorf("repoID required and must be set")
}

type findPushMirrorOptions struct {
	db.ListOptions
	RepoID       int64
	SyncOnCommit optional.Option[bool]
}

func (opts findPushMirrorOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.SyncOnCommit.Has() {
		cond = cond.And(builder.Eq{"sync_on_commit": opts.SyncOnCommit.Value()})
	}
	return cond
}

// GetPushMirrorsByRepoID returns push-mirror information of a repository.
func GetPushMirrorsByRepoID(ctx context.Context, repoID int64, listOptions db.ListOptions) ([]*PushMirror, int64, error) {
	return db.FindAndCount[PushMirror](ctx, findPushMirrorOptions{
		ListOptions: listOptions,
		RepoID:      repoID,
	})
}

func GetPushMirrorByIDAndRepoID(ctx context.Context, id, repoID int64) (*PushMirror, bool, error) {
	var pushMirror PushMirror
	has, err := db.GetEngine(ctx).Where("id = ?", id).And("repo_id = ?", repoID).Get(&pushMirror)
	if !has || err != nil {
		return nil, has, err
	}
	return &pushMirror, true, nil
}

// GetPushMirrorsSyncedOnCommit returns push-mirrors for this repo that should be updated by new commits
func GetPushMirrorsSyncedOnCommit(ctx context.Context, repoID int64) ([]*PushMirror, error) {
	return db.Find[PushMirror](ctx, findPushMirrorOptions{
		RepoID:       repoID,
		SyncOnCommit: optional.Some(true),
	})
}

// PushMirrorsIterate iterates all push-mirror repositories.
func PushMirrorsIterate(ctx context.Context, limit int, f func(idx int, bean any) error) error {
	sess := db.GetEngine(ctx).
		Table("push_mirror").
		Join("INNER", "`repository`", "`repository`.id = `push_mirror`.repo_id").
		Where("`push_mirror`.last_update + (`push_mirror`.`interval` / ?) <= ?", time.Second, time.Now().Unix()).
		And("`push_mirror`.`interval` != 0").
		And("`repository`.is_archived = ?", false).
		OrderBy("last_update ASC")
	if limit > 0 {
		sess = sess.Limit(limit)
	}
	return sess.Iterate(new(PushMirror), f)
}
