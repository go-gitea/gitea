// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/xorm"
	"gopkg.in/ini.v1"
)

// Mirror represents mirror information of a repository.
type Mirror struct {
	ID          int64       `xorm:"pk autoincr"`
	RepoID      int64       `xorm:"INDEX"`
	Repo        *Repository `xorm:"-"`
	Interval    time.Duration
	EnablePrune bool `xorm:"NOT NULL DEFAULT true"`

	UpdatedUnix    util.TimeStamp `xorm:"INDEX"`
	NextUpdateUnix util.TimeStamp `xorm:"INDEX"`

	address string `xorm:"-"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (m *Mirror) BeforeInsert() {
	if m != nil {
		m.UpdatedUnix = util.TimeStampNow()
		m.NextUpdateUnix = util.TimeStampNow()
	}
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (m *Mirror) AfterLoad(session *xorm.Session) {
	if m == nil {
		return
	}

	var err error
	m.Repo, err = getRepositoryByID(session, m.RepoID)
	if err != nil {
		log.Error(3, "getRepositoryByID[%d]: %v", m.ID, err)
	}
}

// ScheduleNextUpdate calculates and sets next update time.
func (m *Mirror) ScheduleNextUpdate() {
	if m.Interval != 0 {
		m.NextUpdateUnix = util.TimeStampNow().AddDuration(m.Interval)
	} else {
		m.NextUpdateUnix = 0
	}
}

func remoteAddress(repoPath string) (string, error) {
	cfg, err := ini.Load(GitConfigPath(repoPath))
	if err != nil {
		return "", err
	}
	return cfg.Section("remote \"origin\"").Key("url").Value(), nil
}

func (m *Mirror) readAddress() {
	if len(m.address) > 0 {
		return
	}
	var err error
	m.address, err = remoteAddress(m.Repo.RepoPath())
	if err != nil {
		log.Error(4, "remoteAddress: %v", err)
	}
}

// Address returns mirror address from Git repository config without credentials.
func (m *Mirror) Address() string {
	m.readAddress()
	return util.SanitizeURLCredentials(m.address, false)
}

// FullAddress returns mirror address from Git repository config.
func (m *Mirror) FullAddress() string {
	m.readAddress()
	return m.address
}

// SaveAddress writes new address to Git repository config.
func (m *Mirror) SaveAddress(addr string) error {
	configPath := m.Repo.GitConfigPath()
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("Load: %v", err)
	}

	cfg.Section("remote \"origin\"").Key("url").SetValue(addr)
	return cfg.SaveToIndent(configPath, "\t")
}

func getMirrorByRepoID(e Engine, repoID int64) (*Mirror, error) {
	m := &Mirror{RepoID: repoID}
	has, err := e.Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrMirrorNotExist
	}
	return m, nil
}

// GetMirrorByRepoID returns mirror information of a repository.
func GetMirrorByRepoID(repoID int64) (*Mirror, error) {
	return getMirrorByRepoID(x, repoID)
}

func updateMirror(e Engine, m *Mirror) error {
	_, err := e.ID(m.ID).AllCols().Update(m)
	return err
}

// UpdateMirror updates the mirror
func UpdateMirror(m *Mirror) error {
	return updateMirror(x, m)
}

// DeleteMirrorByRepoID deletes a mirror by repoID
func DeleteMirrorByRepoID(repoID int64) error {
	_, err := x.Delete(&Mirror{RepoID: repoID})
	return err
}

// IterateNextMirrors iterates mirrors needs to updated
func IterateNextMirrors(f func(idx int, bean interface{}) error) error {
	return x.
		Where("next_update_unix<=?", time.Now().Unix()).
		And("next_update_unix!=0").
		Iterate(new(Mirror), f)
}
