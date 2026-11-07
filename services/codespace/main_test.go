// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func configureGitTransportTestSettings(t *testing.T, protocol string, disableHTTPGit, disableSSH bool, knownHosts []string) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.Repository.DisableHTTPGit, disableHTTPGit))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Disabled, disableSSH))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Domain, "gitea.example.com"))
	t.Cleanup(test.MockVariableValue(&setting.SSH.Port, 22))
	t.Cleanup(test.MockVariableValue(&setting.SSH.StartBuiltinServer, false))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitSSHKnownHosts, knownHosts))
	t.Cleanup(test.MockVariableValue(&setting.Codespace.GitProtocol, protocol))
}
