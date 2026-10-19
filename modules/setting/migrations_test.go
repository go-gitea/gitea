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
	for ini, want := range map[string][4]string{
		`ALLOWED_DOMAINS = github.com
BLOCKED_DOMAINS = gitlab.com
ALLOW_LOCALNETWORKS = true`: {"github.com:*,private:*,loopback:*", "gitlab.com", "strict", ""},
		`ALLOW_LOCALNETWORKS = true`: {"private:*,loopback:*", "", "lax", ""},
		`ALLOWED_HOST_LIST = 10.0.0.0/8
ALLOWED_DOMAINS = github.com
ALLOW_LOCALNETWORKS = true
BLOCKED_HOST_LIST = evil.com
BLOCKED_DOMAINS = gitlab.com`: {"10.0.0.0/8", "evil.com", "lax", ""},
		// an explicit lax mode keeps lax, the deprecated list then only restricts non-public targets
		`ALLOWED_DOMAINS = github.com
EGRESS_MODE = lax`: {"github.com:*", "", "lax", ""},
		`ALLOWED_HOST_LIST = example.com
ALLOWED_INTERNAL_RANGES = 10.0.0.0/8, cgnat`: {"example.com", "", "lax", "10.0.0.0/8, cgnat"},
		// an invalid block entry is logged and dropped, startup continues
		`BLOCKED_HOST_LIST = 10.0.0.5/8`: {"", "10.0.0.5/8", "lax", ""},
	} {
		cfg, err := NewConfigProviderFromData("[migrations]\n" + ini)
		require.NoError(t, err)
		loadMigrationsFrom(cfg)
		assert.Equal(t, want, [4]string{Migrations.AllowedHostList, Migrations.BlockedHostList, Migrations.EgressMode, Migrations.AllowedInternalRanges}, ini)
	}
}
