// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"gitea.dev/modules/log"
)

// removeStaleGnupgLockFiles removes gpg lock files whose holder is gone. gpg only breaks a lock created on the same host,
// but a container gets a new hostname when it is re-created, so a lock left by the previous container blocks gpg forever.
// A lock from another host is treated as stale, so it must only be used for the GnuPG home owned by Gitea.
func removeStaleGnupgLockFiles(gnupgHome string) {
	if runtime.GOOS == "windows" {
		return // gpg on Windows doesn't use pid based lock files
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Warn("Unable to get hostname to check GnuPG lock files: %v", err)
		return
	}
	for _, dir := range []string{gnupgHome, filepath.Join(gnupgHome, "public-keys.d")} { // "public-keys.d" is used by keyboxd
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Warn("Unable to read GnuPG directory %q: %v", dir, err)
			}
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".lock") {
				continue
			}
			lockFile := filepath.Join(dir, name)
			content, err := os.ReadFile(lockFile)
			if err != nil {
				continue
			}
			pid, lockHost, ok := parseGnupgLockFile(string(content))
			// like gpg, a lock naming our own pid is stale, the pid was reused
			if !ok || (lockHost == hostname && pid != os.Getpid() && isProcessAlive(pid)) {
				continue
			}
			if err = os.Remove(lockFile); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Warn("Unable to remove stale GnuPG lock file %q: %v", lockFile, err)
				continue
			}
			log.Info("Removed stale GnuPG lock file %q (pid %d on host %q)", lockFile, pid, lockHost)
		}
	}
}

// parseGnupgLockFile parses the content of a lock file written by gnupg's dotlock: "%10d\n<nodename>\n"
func parseGnupgLockFile(content string) (pid int, hostname string, ok bool) {
	if len(content) < 12 || content[10] != '\n' || content[len(content)-1] != '\n' {
		return 0, "", false
	}
	pid, err := strconv.Atoi(strings.TrimLeft(content[:10], " "))
	hostname = content[11 : len(content)-1]
	if err != nil || pid <= 0 || hostname == "" || strings.Contains(hostname, "\n") {
		return 0, "", false
	}
	return pid, hostname, true
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0)) // EPERM means the process exists but belongs to another user
	_ = proc.Release()
	return !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH)
}
