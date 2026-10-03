// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateDelegateHooksPermissions(t *testing.T) {
	hookDir := t.TempDir()
	existingHookDir := filepath.Join(hookDir, "post-receive.d")
	require.NoError(t, os.MkdirAll(existingHookDir, 0o777))
	require.NoError(t, os.Chmod(existingHookDir, 0o777))

	require.NoError(t, createDelegateHooks(hookDir))

	hookNames, _, _ := getHookTemplates()
	for _, hookName := range hookNames {
		for _, path := range []string{
			filepath.Join(hookDir, hookName),
			filepath.Join(hookDir, hookName+".d"),
			filepath.Join(hookDir, hookName+".d", "gitea"),
		} {
			info, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o755), info.Mode().Perm(), path)
		}
	}
}
