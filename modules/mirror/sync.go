// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/util"
	"github.com/Unknwon/com"
	ini "gopkg.in/ini.v1"
)

const (
	mirrorUpdate = "mirror_update"
)

// MirrorQueue holds an UniqueQueue object of the mirror
var MirrorQueue = sync.NewUniqueQueue(setting.Repository.MirrorQueueLength)

// gitShortEmptySha Git short empty SHA
const gitShortEmptySha = "0000000"

// mirrorSyncResult contains information of a updated reference.
// If the oldCommitID is "0000000", it means a new reference, the value of newCommitID is empty.
// If the newCommitID is "0000000", it means the reference is deleted, the value of oldCommitID is empty.
type mirrorSyncResult struct {
	refName     string
	oldCommitID string
	newCommitID string
}

// parseRemoteUpdateOutput detects create, update and delete operations of references from upstream.
func parseRemoteUpdateOutput(output string) []*mirrorSyncResult {
	results := make([]*mirrorSyncResult, 0, 3)
	lines := strings.Split(output, "\n")
	for i := range lines {
		// Make sure reference name is presented before continue
		idx := strings.Index(lines[i], "-> ")
		if idx == -1 {
			continue
		}

		refName := lines[i][idx+3:]

		switch {
		case strings.HasPrefix(lines[i], " * "): // New reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], " - "): // Delete reference
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				newCommitID: gitShortEmptySha,
			})
		case strings.HasPrefix(lines[i], "   "): // New commits of a reference
			delimIdx := strings.Index(lines[i][3:], " ")
			if delimIdx == -1 {
				log.Error(2, "SHA delimiter not found: %q", lines[i])
				continue
			}
			shas := strings.Split(lines[i][3:delimIdx+3], "..")
			if len(shas) != 2 {
				log.Error(2, "Expect two SHAs but not what found: %q", lines[i])
				continue
			}
			results = append(results, &mirrorSyncResult{
				refName:     refName,
				oldCommitID: shas[0],
				newCommitID: shas[1],
			})

		default:
			log.Warn("parseRemoteUpdateOutput: unexpected update line %q", lines[i])
		}
	}
	return results
}

func remoteAddress(repoPath string) (string, error) {
	cfg, err := ini.Load(models.GitConfigPath(repoPath))
	if err != nil {
		return "", err
	}
	return cfg.Section("remote \"origin\"").Key("url").Value(), nil
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

// runSync returns true if sync finished without error.
func runSync(m *models.Mirror) ([]*mirrorSyncResult, bool) {
	repoPath := m.Repo.RepoPath()
	wikiPath := m.Repo.WikiPath()
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	gitArgs := []string{"remote", "update"}
	if m.EnablePrune {
		gitArgs = append(gitArgs, "--prune")
	}

	_, stderr, err := process.GetManager().ExecDir(
		timeout, repoPath, fmt.Sprintf("Mirror.runSync: %s", repoPath),
		"git", gitArgs...)
	if err != nil {
		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		message, err := sanitizeOutput(stderr, repoPath)
		if err != nil {
			log.Error(4, "sanitizeOutput: %v", err)
			return nil, false
		}
		desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", repoPath, message)
		log.Error(4, desc)
		if err = models.CreateRepositoryNotice(desc); err != nil {
			log.Error(4, "CreateRepositoryNotice: %v", err)
		}
		return nil, false
	}
	output := stderr

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil, false
	}
	if err = models.SyncReleasesWithTags(m.Repo, gitRepo); err != nil {
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
				return nil, false
			}
			desc := fmt.Sprintf("Failed to update mirror wiki repository '%s': %s", wikiPath, message)
			log.Error(4, desc)
			if err = models.CreateRepositoryNotice(desc); err != nil {
				log.Error(4, "CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}

	branches, err := m.Repo.GetBranches()
	if err != nil {
		log.Error(4, "GetBranches: %v", err)
		return nil, false
	}

	for i := range branches {
		cache.Remove(m.Repo.GetCommitsCountCacheKey(branches[i].Name, true))
	}

	m.UpdatedUnix = util.TimeStampNow()
	return parseRemoteUpdateOutput(output), true
}

// MirrorUpdate checks and updates mirror repositories.
func MirrorUpdate() {
	if !models.TaskStartIfNotRunning(mirrorUpdate) {
		return
	}
	defer models.TaskStop(mirrorUpdate)

	log.Trace("Doing: MirrorUpdate")

	if err := models.IterateNextMirrors(func(idx int, bean interface{}) error {
		m := bean.(*models.Mirror)
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

		m, err := models.GetMirrorByRepoID(com.StrTo(repoID).MustInt64())
		if err != nil {
			log.Error(4, "GetMirrorByRepoID [%s]: %v", repoID, err)
			continue
		}

		results, ok := runSync(m)
		if !ok {
			continue
		}

		m.ScheduleNextUpdate()
		if err = models.UpdateMirror(m); err != nil {
			log.Error(4, "UpdateMirror [%s]: %v", repoID, err)
			continue
		}

		var gitRepo *git.Repository
		if len(results) == 0 {
			log.Trace("SyncMirrors [repo_id: %d]: no commits fetched", m.RepoID)
		} else {
			gitRepo, err = git.OpenRepository(m.Repo.RepoPath())
			if err != nil {
				log.Error(2, "OpenRepository [%d]: %v", m.RepoID, err)
				continue
			}
		}

		for _, result := range results {
			// Discard GitHub pull requests, i.e. refs/pull/*
			if strings.HasPrefix(result.refName, "refs/pull/") {
				continue
			}

			// Create reference
			if result.oldCommitID == gitShortEmptySha {
				if err = MirrorSyncCreateAction(m.Repo, result.refName); err != nil {
					log.Error(2, "MirrorSyncCreateAction [repo_id: %d]: %v", m.RepoID, err)
				}
				continue
			}

			// Delete reference
			if result.newCommitID == gitShortEmptySha {
				if err = MirrorSyncDeleteAction(m.Repo, result.refName); err != nil {
					log.Error(2, "MirrorSyncDeleteAction [repo_id: %d]: %v", m.RepoID, err)
				}
				continue
			}

			// Push commits
			oldCommitID, err := git.GetFullCommitID(gitRepo.Path, result.oldCommitID)
			if err != nil {
				log.Error(2, "GetFullCommitID [%d]: %v", m.RepoID, err)
				continue
			}
			newCommitID, err := git.GetFullCommitID(gitRepo.Path, result.newCommitID)
			if err != nil {
				log.Error(2, "GetFullCommitID [%d]: %v", m.RepoID, err)
				continue
			}
			commits, err := gitRepo.CommitsBetweenIDs(newCommitID, oldCommitID)
			if err != nil {
				log.Error(2, "CommitsBetweenIDs [repo_id: %d, new_commit_id: %s, old_commit_id: %s]: %v", m.RepoID, newCommitID, oldCommitID, err)
				continue
			}
			if err = MirrorSyncPushAction(m.Repo, MirrorSyncPushActionOptions{
				RefName:     result.refName,
				OldCommitID: oldCommitID,
				NewCommitID: newCommitID,
				Commits:     models.ListToPushCommits(commits),
			}); err != nil {
				log.Error(2, "MirrorSyncPushAction [repo_id: %d]: %v", m.RepoID, err)
				continue
			}
		}

		// Get latest commit date and update to current repository updated time
		commitDate, err := git.GetLatestCommitTime(m.Repo.RepoPath())
		if err != nil {
			log.Error(2, "GetLatestCommitDate [%s]: %v", m.RepoID, err)
			continue
		}

		if err = models.UpdateRepositoryCols(&models.Repository{
			ID:          m.RepoID,
			UpdatedUnix: util.TimeStamp(commitDate.Unix()),
		}, "updated_unix"); err != nil {
			log.Error(2, "Update repository 'updated_unix' [%s]: %v", m.RepoID, err)
			continue
		}
	}
}

// InitSyncMirrors initializes a go routine to sync the mirrors
func InitSyncMirrors() {
	go SyncMirrors()
}
