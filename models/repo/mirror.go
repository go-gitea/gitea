// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ErrMirrorNotExist mirror does not exist error
var ErrMirrorNotExist = util.NewNotExistErrorf("Mirror does not exist")

// Mirror represents mirror information of a repository.
type Mirror struct {
	ID          int64       `xorm:"pk autoincr"`
	RepoID      int64       `xorm:"INDEX"`
	Repo        *Repository `xorm:"-"`
	Interval    time.Duration
	EnablePrune bool `xorm:"NOT NULL DEFAULT true"`

	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX"`
	NextUpdateUnix timeutil.TimeStamp `xorm:"INDEX"`
	LastSyncUnix   timeutil.TimeStamp `xorm:"INDEX"`

	LFS         bool   `xorm:"lfs_enabled NOT NULL DEFAULT false"`
	LFSEndpoint string `xorm:"lfs_endpoint TEXT"`

	RemoteAddress          string `xorm:"VARCHAR(2048)"`
	RemoteAddressEncrypted string `xorm:"TEXT"` // only set when the address has credentials, they are kept out of the git config
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

// SetRemoteAddressWithCredentials stores the address including its credentials, encrypted
func (m *Mirror) SetRemoteAddressWithCredentials(addr string) (err error) {
	m.RemoteAddressEncrypted, err = encryptRemoteAddress(addr)
	return err
}

// GetRemoteAddressWithCredentials returns the address including its credentials
func (m *Mirror) GetRemoteAddressWithCredentials(ctx context.Context) (string, error) {
	return decryptRemoteAddress(ctx, m.RemoteAddressEncrypted, m.GetRepository(ctx), m.GetRemoteName())
}

func encryptRemoteAddress(addr string) (string, error) {
	if git.RemoteAddressWithoutCredentials(addr) == addr {
		return "", nil
	}
	return secret.EncryptSecret(setting.SecretKey, addr)
}

func decryptRemoteAddress(ctx context.Context, encrypted string, repo *Repository, remoteName string) (string, error) {
	if encrypted != "" {
		return secret.DecryptSecret(setting.SecretKey, encrypted)
	}
	if repo == nil {
		return "", ErrMirrorNotExist
	}
	// the address has no credentials, or they have not been moved out of the git config yet
	return git.GetRemoteAddress(ctx, repo, remoteName)
}

// UpdateMirror updates the mirror
func UpdateMirror(ctx context.Context, m *Mirror) error {
	_, err := db.GetEngine(ctx).ID(m.ID).AllCols().Update(m)
	return err
}

// UpdateMirrorRemoteAddressEncrypted updates the encrypted remote address of the mirror
func UpdateMirrorRemoteAddressEncrypted(ctx context.Context, m *Mirror) error {
	_, err := db.GetEngine(ctx).ID(m.ID).Cols("remote_address_encrypted").Update(m)
	return err
}

// TouchMirror updates the mirror updatedUnix
func TouchMirror(ctx context.Context, m *Mirror) error {
	m.UpdatedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(ctx).ID(m.ID).Cols("updated_unix").Update(m)
	return err
}

// DeleteMirrorByRepoID deletes a mirror by repoID
func DeleteMirrorByRepoID(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Delete(&Mirror{RepoID: repoID})
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
