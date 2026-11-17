// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
)

// StaleLockFileMinAge is how old a lock file must be to be considered stale.
// A git process holds a lock file only for a very short time, so a lock file that has not been
// touched for this long is the leftover of a git process which was killed or canceled.
const StaleLockFileMinAge = time.Hour

var looseObjectDirRegexp = regexp.MustCompile(`^[0-9a-f]{2}$`)

// RemoveStaleLockFiles removes the stale "*.lock" files (for example "refs/heads/main.lock", "config.lock",
// "packed-refs.lock", "shallow.lock") from a bare repository, and returns the number of removed files.
//
// Lock file names are not hard-coded because git creates them for many different files (and refs).
// Instead, every regular "*.lock" file whose modification time is older than minAge is removed.
// The loose object fan-out directories and the hooks directory are skipped, git never creates lock files
// there, and they can be huge or contain user-provided files.
func RemoveStaleLockFiles(repo gitrepo.RepositoryFacade, minAge time.Duration) (removed int, err error) {
	repoPath := gitrepo.RepoLocalPath(repo)
	if _, err = os.Stat(repoPath); os.IsNotExist(err) {
		return 0, nil
	}
	deadline := time.Now().Add(-minAge)
	err = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil // a file or directory removed while we are walking, e.g. by a running git process
			}
			return walkErr
		}
		if d.IsDir() {
			if path != repoPath && (d.Name() == "hooks" || d.Name() == "custom_hooks" || looseObjectDirRegexp.MatchString(d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || filepath.Ext(d.Name()) != ".lock" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.ModTime().After(deadline) {
			return nil
		}
		if err = os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		log.Info("Removed stale git lock file %s", path)
		removed++
		return nil
	})
	return removed, err
}
