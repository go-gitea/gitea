// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"net"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMigrateWhiteBlocklist(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})
	nonAdminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	setting.Migrations.AllowedDomains = "github.com"
	setting.Migrations.AllowLocalNetworks = false
	assert.NoError(t, Init())

	err := IsMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git", nonAdminUser)
	assert.Error(t, err)

	err = IsMigrateURLAllowed("https://github.com/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed("https://gITHUb.com/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	setting.Migrations.AllowedDomains = ""
	setting.Migrations.BlockedDomains = "github.com"
	assert.NoError(t, Init())

	err = IsMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git", nonAdminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed("https://github.com/go-gitea/gitea.git", nonAdminUser)
	assert.Error(t, err)

	err = IsMigrateURLAllowed("https://10.0.0.1/go-gitea/gitea.git", nonAdminUser)
	assert.Error(t, err)

	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, Init())
	err = IsMigrateURLAllowed("https://10.0.0.1/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = false

	err = IsMigrateURLAllowed("/home/foo/bar/goo", adminUser)
	assert.Error(t, err)

	setting.ImportLocalPaths = true
	abs, err := filepath.Abs(".")
	assert.NoError(t, err)

	err = IsMigrateURLAllowed(abs, adminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed(abs, nonAdminUser)
	assert.Error(t, err)

	nonAdminUser.AllowImportLocal = true
	err = IsMigrateURLAllowed(abs, nonAdminUser)
	assert.NoError(t, err)

	setting.ImportLocalPaths = old
}

func TestAllowBlockList(t *testing.T) {
	init := func(allow, block string, local bool) {
		setting.Migrations.AllowedDomains = allow
		setting.Migrations.BlockedDomains = block
		setting.Migrations.AllowLocalNetworks = local
		assert.NoError(t, Init())
	}

	// default, allow all external, block none, no local networks
	init("", "", false)
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// allow all including local networks (it could lead to SSRF in production)
	init("", "", true)
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// allow wildcard, block some subdomains. if the domain name is allowed, then the local network check is skipped
	init("*.domain.com", "blocked.domain.com", false)
	assert.NoError(t, checkByAllowBlockList("sub.domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.NoError(t, checkByAllowBlockList("sub.domain.com", []net.IP{net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList("blocked.domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("sub.other.com", []net.IP{net.ParseIP("1.2.3.4")}))

	// allow wildcard (it could lead to SSRF in production)
	init("*", "", false)
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// local network can still be blocked
	init("*", "127.0.0.*", false)
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// reset
	init("", "", false)
}
