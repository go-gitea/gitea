// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ErrMirrorNotExist mirror does not exist error
var ErrMirrorNotExist = util.NewNotExistErrorf("Mirror does not exist")

var ErrReadOnlyMirror = util.NewPermissionDeniedErrorf("the repository is a read-only mirror")

// IsPersonalIssueState reports whether a request path only changes the doer's own state on an issue,
// like watching it or marking files viewed, which stays allowed on a read-only mirror
func IsPersonalIssueState(path string) bool {
	return strings.HasSuffix(path, "/watch") || strings.HasSuffix(path, "/viewed-files") || strings.Contains(path, "/subscriptions")
}

// Mirror represents mirror information of a repository.
type Mirror struct {
	ID          int64       `xorm:"pk autoincr"`
	RepoID      int64       `xorm:"INDEX"`
	Repo        *Repository `xorm:"-"`
	Interval    time.Duration
	EnablePrune bool `xorm:"NOT NULL DEFAULT true"`

	SyncIssues       bool `xorm:"NOT NULL DEFAULT false"`
	SyncPullRequests bool `xorm:"NOT NULL DEFAULT false"`
	SyncTopics       bool `xorm:"NOT NULL DEFAULT false"`

	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX"`
	NextUpdateUnix timeutil.TimeStamp `xorm:"INDEX"`
	LastSyncUnix   timeutil.TimeStamp `xorm:"INDEX"`

	LFS         bool   `xorm:"lfs_enabled NOT NULL DEFAULT false"`
	LFSEndpoint string `xorm:"lfs_endpoint TEXT"`

	RemoteAddress string `xorm:"VARCHAR(2048)"`
}

// SyncsMetadata reports whether this mirror syncs anything on top of git
func (m *Mirror) SyncsMetadata() bool {
	return m.SyncIssues || m.SyncPullRequests || m.SyncTopics
}

// SyncsIssuesOrPulls reports whether this mirror's issues and pull requests are read-only
func (m *Mirror) SyncsIssuesOrPulls() bool {
	return m.SyncIssues || m.SyncPullRequests
}

func init() {
	db.RegisterModel(new(Mirror))
}

// BeforeInsert will be invoked by XORM before inserting a record
func (m *Mirror) BeforeInsert() {
	if m != nil {
		m.UpdatedUnix = timeutil.TimeStampNow()
		m.NextUpdateUnix = timeutil.TimeStampNow()
	}
}

// GetRepository returns the repository.
func (m *Mirror) GetRepository(ctx context.Context) *Repository {
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
func (m *Mirror) GetRemoteName() string {
	return "origin"
}

// ScheduleNextUpdate calculates and sets next update time.
func (m *Mirror) ScheduleNextUpdate() {
	if m.Interval != 0 {
		m.NextUpdateUnix = timeutil.TimeStampNow().AddDuration(m.Interval)
	} else {
		m.NextUpdateUnix = 0
	}
}

// GetMirrorByRepoID returns mirror information of a repository.
func GetMirrorByRepoID(ctx context.Context, repoID int64) (*Mirror, error) {
	m, has, err := db.Get[Mirror](ctx, builder.Eq{"repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMirrorNotExist
	}
	return m, nil
}

// UpdateMirror updates the mirror
func UpdateMirror(ctx context.Context, m *Mirror) error {
	_, err := db.GetEngine(ctx).ID(m.ID).AllCols().Update(m)
	return err
}

// TouchMirror updates the mirror updatedUnix
func TouchMirror(ctx context.Context, m *Mirror) error {
	m.UpdatedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(ctx).ID(m.ID).Cols("updated_unix").Update(m)
	return err
}

// DeleteRepoPullMirror deletes the repo's mirror (pull mirror)
func DeleteRepoPullMirror(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id=?", repoID).Delete(&Mirror{})
	return err
}

// MirrorsIterate iterates all mirror repositories.
func MirrorsIterate(ctx context.Context, limit int, f func(idx int, bean any) error) error {
	sess := db.GetEngine(ctx).
		Where("next_update_unix<=?", time.Now().Unix()).
		And("next_update_unix!=0").
		OrderBy("updated_unix ASC")
	if limit > 0 {
		sess = sess.Limit(limit)
	}
	return sess.Iterate(new(Mirror), f)
}

// InsertMirror inserts a mirror to database
func InsertMirror(ctx context.Context, mirror *Mirror) error {
	_, err := db.GetEngine(ctx).Insert(mirror)
	return err
}
