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
	for ini, want := range map[string][3]string{
		`ALLOWED_DOMAINS = github.com
BLOCKED_DOMAINS = gitlab.com
ALLOW_LOCALNETWORKS = true`: {"github.com:*,private:*,loopback:*", "gitlab.com", "strict"},
		`ALLOW_LOCALNETWORKS = true`: {"private:*,loopback:*", "", "lax"},
		`ALLOWED_HOST_LIST = 10.0.0.0/8
ALLOWED_DOMAINS = github.com
ALLOW_LOCALNETWORKS = true
BLOCKED_HOST_LIST = evil.com
BLOCKED_DOMAINS = gitlab.com`: {"10.0.0.0/8", "evil.com", "lax"},
	} {
		cfg, err := NewConfigProviderFromData("[migrations]\n" + ini)
		require.NoError(t, err)
		loadMigrationsFrom(cfg)
		assert.Equal(t, want, [3]string{Migrations.AllowedHostList, Migrations.BlockedHostList, Migrations.EgressMode}, ini)
	}
}
