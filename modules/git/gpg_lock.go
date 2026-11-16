// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gitea.dev/modules/log"
)

// removeStaleKeyboxdLock removes keyboxd's lock left by another host (a re-created container) or naming our reused pid, gpg breaks neither
func removeStaleKeyboxdLock(gnupgHome string) {
	lockFile := filepath.Join(gnupgHome, "public-keys.d", "pubring.db.lock")
	content, err := os.ReadFile(lockFile)
	if err != nil {
		return
	}
	pidField, lockHost, _ := strings.Cut(strings.TrimSuffix(string(content), "\n"), "\n") // gpg's dotlock writes "%10d\n<nodename>\n"
	pid, err := strconv.Atoi(strings.TrimSpace(pidField))
	if err != nil {
		return
	}
	hostname, err := os.Hostname()
	if err != nil || (lockHost == hostname && pid != os.Getpid()) {
		return
	}
	if err = os.Remove(lockFile); err != nil {
		log.Warn("Unable to remove stale GnuPG lock file %q: %v", lockFile, err)
		return
	}
	log.Info("Removed stale GnuPG lock file %q (pid %d on host %q)", lockFile, pid, lockHost)
}
