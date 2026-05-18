// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagedSSHCommand(t *testing.T) {
	origMode := setting.Migrations.SSHHostKeyChecking
	origData := setting.AppDataPath
	t.Cleanup(func() {
		setting.Migrations.SSHHostKeyChecking = origMode
		setting.AppDataPath = origData
	})

	t.Run("accept-new persists to managed known_hosts", func(t *testing.T) {
		dataPath := t.TempDir()
		setting.AppDataPath = dataPath
		setting.Migrations.SSHHostKeyChecking = "accept-new"

		knownHosts := filepath.Join(dataPath, "home", ".ssh", "known_hosts")
		expected := "ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=" + util.ShellEscape(knownHosts)
		assert.Equal(t, expected, managedSSHCommand())

		// the known_hosts directory must be created so ssh can write to it
		info, err := os.Stat(filepath.Dir(knownHosts))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("yes is strict and still uses managed known_hosts", func(t *testing.T) {
		dataPath := t.TempDir()
		setting.AppDataPath = dataPath
		setting.Migrations.SSHHostKeyChecking = "yes"

		knownHosts := filepath.Join(dataPath, "home", ".ssh", "known_hosts")
		expected := "ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=" + util.ShellEscape(knownHosts)
		assert.Equal(t, expected, managedSSHCommand())
	})

	t.Run("no disables verification with /dev/null known_hosts", func(t *testing.T) {
		setting.AppDataPath = t.TempDir()
		setting.Migrations.SSHHostKeyChecking = "no"

		expected := "ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=" + util.ShellEscape(os.DevNull)
		assert.Equal(t, expected, managedSSHCommand())
	})

	t.Run("empty AppDataPath falls back without UserKnownHostsFile", func(t *testing.T) {
		setting.AppDataPath = ""
		setting.Migrations.SSHHostKeyChecking = "accept-new"

		assert.Equal(t, "ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new", managedSSHCommand())
	})

	t.Run("BatchMode is always set so the worker never hangs", func(t *testing.T) {
		setting.AppDataPath = t.TempDir()
		for _, mode := range []string{"accept-new", "yes", "no"} {
			setting.Migrations.SSHHostKeyChecking = mode
			assert.Contains(t, managedSSHCommand(), "ssh -o BatchMode=yes ", "mode %q must run ssh non-interactively", mode)
		}
	})
}
