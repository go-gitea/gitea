// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	"gopkg.in/ini.v1"
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
	m.NextUpdateUnix = util.TimeStampNow().AddDuration(m.Interval)
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

// sanitizeOutput sanitizes output of a command, replacing occurrences of the
// repository's remote address with a sanitized version.
func sanitizeOutput(output, repoPath string) (string, error) {
	remoteAddr, err := remoteAddress(repoPath)
	if err != nil {
		// if we're unable to load the remote address, then we're unable to
		// sanitize.
		return "", err
	}
	return util.SanitizeMessage(output, remoteAddr), nil
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
		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		message, err := sanitizeOutput(stderr, repoPath)
		if err != nil {
			log.Error(4, "sanitizeOutput: %v", err)
			return false
		}
		desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", repoPath, message)
		log.Error(4, desc)
		if err = CreateRepositoryNotice(desc); err != nil {
			log.Error(4, "CreateRepositoryNotice: %v", err)
		}
		return false
	}

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return false
	}
	if err = SyncReleasesWithTags(m.Repo, gitRepo); err != nil {
		log.Error(4, "Failed to synchronize tags to releases for repository: %v", err)
	}

	if err := m.Repo.UpdateSize(); err != nil {
		log.Error(4, "Failed to update size for mirror repository: %v", err)
	}

	if m.Repo.HasWiki() {
		if _, stderr, err := process.GetManager().ExecDir(
			timeout, wikiPath, fmt.Sprintf("Mirror.runSync: %s", wikiPath),
			"git", "remote", "update", "--prune"); err != nil {
			// sanitize the output, since it may contain the remote address, which may
			// contain a password
			message, err := sanitizeOutput(stderr, wikiPath)
			if err != nil {
				log.Error(4, "sanitizeOutput: %v", err)
				return false
			}
			desc := fmt.Sprintf("Failed to update mirror wiki repository '%s': %s", wikiPath, message)
			log.Error(4, desc)
			if err = CreateRepositoryNotice(desc); err != nil {
				log.Error(4, "CreateRepositoryNotice: %v", err)
			}
			return false
		}
	}

	m.UpdatedUnix = util.TimeStampNow()
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
