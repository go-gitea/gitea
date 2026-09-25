// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMigrationsFrom(t *testing.T) {
	defer test.MockVariableValue(&Migrations)()
	for ini, want := range map[string][2]string{
		``: {"external", ""},
		`ALLOWED_DOMAINS = github.com
BLOCKED_DOMAINS = gitlab.com
ALLOW_LOCALNETWORKS = true`: {"github.com,private,loopback", "gitlab.com"},
		`ALLOWED_HOST_LIST = external, 10.0.0.0/8
ALLOWED_DOMAINS = github.com
BLOCKED_HOST_LIST = evil.com
BLOCKED_DOMAINS = gitlab.com`: {"external, 10.0.0.0/8", "evil.com"},
	} {
		cfg, err := NewConfigProviderFromData("[migrations]\n" + ini)
		require.NoError(t, err)
		loadMigrationsFrom(cfg)
		assert.Equal(t, want, [2]string{Migrations.AllowedHostList, Migrations.BlockedHostList}, ini)
	}
}
