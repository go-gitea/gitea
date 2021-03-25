// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/util"
)

// mirrorQueue holds an UniqueQueue object of the mirror
var mirrorQueue = sync.NewUniqueQueue(setting.Repository.MirrorQueueLength)

func readAddress(m *models.Mirror) {
	if len(m.Address) > 0 {
		return
	}
	var err error
	m.Address, err = remoteAddress(m.Repo.RepoPath())
	if err != nil {
		log.Error("remoteAddress: %v", err)
	}
}

func remoteAddress(repoPath string) (string, error) {
	var cmd *git.Command
	err := git.LoadGitVersion()
	if err != nil {
		return "", err
	}
	if git.CheckGitVersionAtLeast("2.7") == nil {
		cmd = git.NewCommand("remote", "get-url", "origin")
	} else {
		cmd = git.NewCommand("config", "--get", "remote.origin.url")
	}

	result, err := cmd.RunInDir(repoPath)
	if err != nil {
		if strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return "", nil
		}
		return "", err
	}
	if len(result) > 0 {
		return result[:len(result)-1], nil
	}
	return "", nil
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

// AddressNoCredentials returns mirror address from Git repository config without credentials.
func AddressNoCredentials(m *models.Mirror) string {
	readAddress(m)
	u, err := url.Parse(m.Address)
	if err != nil {
		// this shouldn't happen but just return it unsanitised
		return m.Address
	}
	u.User = nil
	return u.String()
}

// UpdateAddress writes new address to Git repository and database
func UpdateAddress(m *models.Mirror, addr string) error {
	repoPath := m.Repo.RepoPath()
	// Remove old origin
	_, err := git.NewCommand("remote", "rm", "origin").RunInDir(repoPath)
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return err
	}

	_, err = git.NewCommand("remote", "add", "origin", "--mirror=fetch", addr).RunInDir(repoPath)
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return err
	}

	if m.Repo.HasWiki() {
		wikiPath := m.Repo.WikiPath()
		wikiRemotePath := repo_module.WikiRemoteURL(addr)
		// Remove old origin of wiki
		_, err := git.NewCommand("remote", "rm", "origin").RunInDir(wikiPath)
		if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return err
		}

		_, err = git.NewCommand("remote", "add", "origin", "--mirror=fetch", wikiRemotePath).RunInDir(wikiPath)
		if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
			return err
		}
	}

	m.Repo.OriginalURL = addr
	return models.UpdateRepositoryCols(m.Repo, "original_url")
}

// Address returns mirror address from Git repository config without credentials.
func Address(m *models.Mirror) string {
	readAddress(m)
	return util.SanitizeURLCredentials(m.Address, false)
}

// Username returns the mirror address username
func Username(m *models.Mirror) string {
	readAddress(m)
	u, err := url.Parse(m.Address)
	if err != nil {
		// this shouldn't happen but if it does return ""
		return ""
	}
	return u.User.Username()
}

// Password returns the mirror address password
func Password(m *models.Mirror) string {
	readAddress(m)
	u, err := url.Parse(m.Address)
	if err != nil {
		// this shouldn't happen but if it does return ""
		return ""
	}
	password, _ := u.User.Password()
	return password
}

// Update checks and updates mirror repositories.
func Update(ctx context.Context) error {
	log.Trace("Doing: Update")

	handler := func(idx int, bean interface{}) error {
		var item string
		if m, ok := bean.(*models.Mirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected mirror found: %d", m.ID)
				return nil
			}
			item = fmt.Sprintf("pull %d", m.RepoID)
		} else if m, ok := bean.(*models.PushMirror); ok {
			if m.Repo == nil {
				log.Error("Disconnected push-mirror found: %d", m.ID)
				return nil
			}
			item = fmt.Sprintf("push %d", m.ID)
		} else {
			log.Error("Unknown bean: %v", bean)
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted")
		default:
			mirrorQueue.Add(item)
			return nil
		}
	}

	if err := models.MirrorsIterate(handler); err != nil {
		log.Trace("MirrorsIterate: %v", err)
		return err
	}
	if err := models.PushMirrorsIterate(handler); err != nil {
		log.Trace("PushMirrorsIterate: %v", err)
		return err
	}
	log.Trace("Finished: Update")
	return nil
}

// syncMirrors checks and syncs mirrors.
// FIXME: graceful: this should be a persistable queue
func syncMirrors(ctx context.Context) {
	// Start listening on new sync requests.
	for {
		select {
		case <-ctx.Done():
			mirrorQueue.Close()
			return
		case item := <-mirrorQueue.Queue():
			if strings.HasPrefix(item, "pull") {
				syncPullMirror(item[5:])
			} else if strings.HasPrefix(item, "push") {
				//syncPushMirror(item[5:])
			} else {
				log.Error("Unknown item in queue: %v", item)
			}
			mirrorQueue.Remove(item)
		}
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	go graceful.GetManager().RunWithShutdownContext(syncMirrors)
}

// StartToMirror adds repoID to mirror queue
func StartToMirror(repoID int64) {
	go mirrorQueue.Add(fmt.Sprintf("pull %d", repoID))
}
