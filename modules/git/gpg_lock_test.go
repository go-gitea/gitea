// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveStaleKeyboxdLock(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	gnupgHome := t.TempDir()
	lockFile := filepath.Join(gnupgHome, "public-keys.d", "pubring.db.lock")
	require.NoError(t, os.Mkdir(filepath.Dir(lockFile), 0o700))

	for content, stale := range map[string]bool{
		fmt.Sprintf("%10d\n%s-old\n", os.Getpid()+1, hostname): true,
		fmt.Sprintf("%10d\n%s\n", os.Getpid(), hostname):       true,
		fmt.Sprintf("%10d\n%s\n", os.Getpid()+1, hostname):     false,
		"not a gnupg lock file":                                false,
	} {
		require.NoError(t, os.WriteFile(lockFile, []byte(content), 0o600))
		removeStaleKeyboxdLock(gnupgHome)
		if stale {
			assert.NoFileExists(t, lockFile, "content: %q", content)
		} else {
			assert.FileExists(t, lockFile, "content: %q", content)
		}
	}
}
