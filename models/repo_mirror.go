// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	"gopkg.in/ini.v1"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
)

// MirrorQueue holds an UniqueQueue object of the mirror
var MirrorQueue = sync.NewUniqueQueue(setting.Repository.MirrorQueueLength)

// Mirror represents mirror information of a repository.
type Mirror struct {
	ID          int64       `xorm:"pk autoincr"`
	RepoID      int64       `xorm:"INDEX"`
	Repo        *Repository `xorm:"-"`
	Interval    time.Duration
	EnablePrune bool `xorm:"NOT NULL DEFAULT true"`

	Updated        time.Time `xorm:"-"`
	UpdatedUnix    int64     `xorm:"INDEX"`
	NextUpdate     time.Time `xorm:"-"`
	NextUpdateUnix int64     `xorm:"INDEX"`

	address string `xorm:"-"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (m *Mirror) BeforeInsert() {
	m.UpdatedUnix = time.Now().Unix()
	m.NextUpdateUnix = m.NextUpdate.Unix()
}

// BeforeUpdate is invoked from XORM before updating this object.
func (m *Mirror) BeforeUpdate() {
	m.UpdatedUnix = time.Now().Unix()
	m.NextUpdateUnix = m.NextUpdate.Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (m *Mirror) AfterSet(colName string, _ xorm.Cell) {
	var err error
	switch colName {
	case "repo_id":
		m.Repo, err = GetRepositoryByID(m.RepoID)
		if err != nil {
			log.Error(3, "GetRepositoryByID[%d]: %v", m.ID, err)
		}
	case "updated_unix":
		m.Updated = time.Unix(m.UpdatedUnix, 0).Local()
	case "next_update_unix":
		m.NextUpdate = time.Unix(m.NextUpdateUnix, 0).Local()
	}
}

// ScheduleNextUpdate calculates and sets next update time.
func (m *Mirror) ScheduleNextUpdate() {
	m.NextUpdate = time.Now().Add(m.Interval)
}

func (m *Mirror) readAddress() {
	if len(m.address) > 0 {
		return
	}

	cfg, err := ini.Load(m.Repo.GitConfigPath())
	if err != nil {
		log.Error(4, "Load: %v", err)
		return
	}
	m.address = cfg.Section("remote \"origin\"").Key("url").Value()
}

// HandleCloneUserCredentials replaces user credentials from HTTP/HTTPS URL
// with placeholder <credentials>.
// It will fail for any other forms of clone addresses.
func HandleCloneUserCredentials(url string, mosaics bool) string {
	i := strings.Index(url, "@")
	if i == -1 {
		return url
	}
	start := strings.Index(url, "://")
	if start == -1 {
		return url
	}
	if mosaics {
		return url[:start+3] + "<credentials>" + url[i:]
	}
	return url[:start+3] + url[i+1:]
}

// Address returns mirror address from Git repository config without credentials.
func (m *Mirror) Address() string {
	m.readAddress()
	return HandleCloneUserCredentials(m.address, false)
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

// runSync returns true if sync finished without error.
func (m *Mirror) runSync() bool {
	repoPath := m.Repo.RepoPath()
	wikiPath := m.Repo.WikiPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	gitArgs := []string{"remote", "update"}
	if m.EnablePrune {
		gitArgs = append(gitArgs, "--prune")
	}

	if _, stderr, err := process.GetManager().ExecDir(
		timeout, repoPath, fmt.Sprintf("Mirror.runSync: %s", repoPath),
		"git", gitArgs...); err != nil {
		desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", repoPath, stderr)
		log.Error(4, desc)
		if err = CreateRepositoryNotice(desc); err != nil {
			log.Error(4, "CreateRepositoryNotice: %v", err)
		}
		return false
	}

	if err := m.Repo.UpdateSize(); err != nil {
		log.Error(4, "Failed to update size for mirror repository: %v", err)
	}

	if m.Repo.HasWiki() {
		if _, stderr, err := process.GetManager().ExecDir(
			timeout, wikiPath, fmt.Sprintf("Mirror.runSync: %s", wikiPath),
			"git", "remote", "update", "--prune"); err != nil {
			desc := fmt.Sprintf("Failed to update mirror wiki repository '%s': %s", wikiPath, stderr)
			log.Error(4, desc)
			if err = CreateRepositoryNotice(desc); err != nil {
				log.Error(4, "CreateRepositoryNotice: %v", err)
			}
			return false
		}
	}

	return true
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
	_, err := e.Id(m.ID).AllCols().Update(m)
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

// MirrorUpdate checks and updates mirror repositories.
func MirrorUpdate() {
	if !taskStatusTable.StartIfNotRunning(mirrorUpdate) {
		return
	}
	defer taskStatusTable.Stop(mirrorUpdate)

	log.Trace("Doing: MirrorUpdate")

	if err := x.
		Where("next_update_unix<=?", time.Now().Unix()).
		Iterate(new(Mirror), func(idx int, bean interface{}) error {
			m := bean.(*Mirror)
			if m.Repo == nil {
				log.Error(4, "Disconnected mirror repository found: %d", m.ID)
				return nil
			}

			MirrorQueue.Add(m.RepoID)
			return nil
		}); err != nil {
		log.Error(4, "MirrorUpdate: %v", err)
	}
}

// SyncMirrors checks and syncs mirrors.
// TODO: sync more mirrors at same time.
func SyncMirrors() {
	// Start listening on new sync requests.
	for repoID := range MirrorQueue.Queue() {
		log.Trace("SyncMirrors [repo_id: %v]", repoID)
		MirrorQueue.Remove(repoID)

		m, err := GetMirrorByRepoID(com.StrTo(repoID).MustInt64())
		if err != nil {
			log.Error(4, "GetMirrorByRepoID [%s]: %v", repoID, err)
			continue
		}

		if !m.runSync() {
			continue
		}

		m.ScheduleNextUpdate()
		if err = UpdateMirror(m); err != nil {
			log.Error(4, "UpdateMirror [%s]: %v", repoID, err)
			continue
		}
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	go SyncMirrors()
}
