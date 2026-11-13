// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveStaleGnupgLockFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("gpg on Windows doesn't use pid based lock files")
	}

	hostname, err := os.Hostname()
	require.NoError(t, err)

	cmd := exec.Command("git", "version")
	require.NoError(t, cmd.Run())
	deadPid := cmd.Process.Pid // the process has exited and been reaped
	livePid := os.Getppid()

	lockContent := func(pid int, host string) string {
		return fmt.Sprintf("%10d\n%s\n", pid, host) // the format written by gnupg's dotlock
	}

	gnupgHome := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(gnupgHome, "public-keys.d"), 0o700))
	files := map[string]struct {
		content string
		stale   bool
	}{
		"pubring.kbx.lock":                   {lockContent(livePid, "old-container-id"), true},
		"public-keys.d/pubring.db.lock":      {lockContent(deadPid, hostname), true},
		"trustdb.gpg.lock":                   {lockContent(os.Getpid(), hostname), true},
		"public-keys.d/keyboxd.lock":         {lockContent(livePid, hostname), false},
		"random_seed.lock":                   {"not a gnupg lock file", false},
		"empty.lock":                         {"", false},
		"zero-pid.lock":                      {lockContent(0, "old-container-id"), false},
		".#lk0x0000aaaa.old-container-id.42": {lockContent(deadPid, "old-container-id"), false}, // not a lock, gpg hard-links locks from such files
	}
	for name, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(gnupgHome, name), []byte(f.content), 0o600))
	}

	removeStaleGnupgLockFiles(gnupgHome)

	for name, f := range files {
		if f.stale {
			assert.NoFileExists(t, filepath.Join(gnupgHome, name), "content: %q", f.content)
		} else {
			assert.FileExists(t, filepath.Join(gnupgHome, name), "content: %q", f.content)
		}
	}

	// a missing gnupg home is not an error
	removeStaleGnupgLockFiles(filepath.Join(gnupgHome, "no-such-dir"))
}
