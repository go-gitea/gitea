// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
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

func TestGiteaDelegateHooksSkipInternalPush(t *testing.T) {
	defer test.MockVariableValue(&setting.AppPath, "false")()
	for _, scriptType := range []string{"bash", "sh"} {
		t.Run(scriptType, func(t *testing.T) {
			defer test.MockVariableValue(&setting.ScriptType, scriptType)()
			hookDir := t.TempDir()
			require.NoError(t, createDelegateHooks(hookDir))
			for hookName, skipsInternal := range map[string]bool{"pre-receive": true, "update": true, "post-receive": false} {
				runHook := func(env ...string) error {
					cmd := exec.Command(filepath.Join(hookDir, hookName+".d", "gitea"))
					cmd.Env = append(os.Environ(), env...)
					return cmd.Run()
				}
				assert.Error(t, runHook(), hookName)
				assert.Equal(t, skipsInternal, runHook("GITEA_INTERNAL_PUSH=true") == nil, hookName)
			}
		})
	}
}
