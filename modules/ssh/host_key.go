// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"os"
	"path/filepath"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// managedSSHCommand builds the GIT_SSH_COMMAND used for Gitea-managed SSH
// operations (migration / mirror with a generated keypair). ssh runs
// non-interactively (BatchMode) so the worker never hangs on an unknown host,
// and the configured host-key policy is applied. The ssh executable is
// configurable (Migrations.SSHCommand) for hosts where "ssh" is not on PATH.
func managedSSHCommand(identityFile string) string {
	ssh := util.ShellEscape(setting.Migrations.SSHCommand)
	var cmd string
	mode := setting.Migrations.SSHHostKeyChecking
	if mode == "no" {
		cmd = ssh + " -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=" + util.ShellEscape(os.DevNull)
	} else {
		cmd = ssh + " -o BatchMode=yes -o StrictHostKeyChecking=" + mode
		// Persist accepted host keys in a Gitea-managed file so a later key change
		// is detected (TOFU); fall back to ssh's default known_hosts if unset.
		if setting.AppDataPath != "" {
			knownHosts := filepath.Join(setting.AppDataPath, "home", ".ssh", "known_hosts")
			if err := os.MkdirAll(filepath.Dir(knownHosts), 0o700); err == nil {
				cmd += " -o UserKnownHostsFile=" + util.ShellEscape(knownHosts)
			}
		}
	}
	// pin auth to the managed key so ssh ignores the OS user's $HOME/.ssh identities
	if identityFile != "" {
		cmd += " -o IdentitiesOnly=yes -i " + util.ShellEscape(identityFile)
	}
	return cmd
}
