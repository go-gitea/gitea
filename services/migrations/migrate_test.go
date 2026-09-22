// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
)

// migrationTestPolicy mirrors what egress.GetMigrationPolicy builds from the migration
// settings for a given ALLOWED_DOMAINS / BLOCKED_DOMAINS / ALLOW_LOCALNETWORKS combination.
func migrationTestPolicy(allow, block string) *policy.Policy {
	return policy.NewPolicy("git-proxy",
		policy.WithAllow(allow, "migrations.ALLOWED_DOMAINS/ALLOW_LOCALNETWORKS"),
		policy.WithBlock(block, "migrations.BLOCKED_DOMAINS"))
}

func TestIsAuthenticationError(t *testing.T) {
	errDummy := errors.New("dummy")
	cases := []struct {
		name string
		want bool
		err  error
	}{
		{"git authentication failed", true, gitcmd.NewRunStdError(errDummy, "fatal: Authentication failed for 'https://host/repo.git/'")},
		{"git could not read username", true, fmt.Errorf("%w", gitcmd.NewRunStdError(errDummy, "fatal: could not read Username for 'https://host'"))},
		{"github unauthorized", true, util.SanitizeErrorCredentialURLs(&github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusUnauthorized}})},
		{"github other", false, &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusNotFound}}},
		{"github nil response", false, &github.ErrorResponse{}},
		{"unrelated error", false, errDummy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsAuthenticationError(c.err))
		})
	}
}

func TestMigrateWhiteBlocklist(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})
	nonAdminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	// ALLOWED_DOMAINS=github.com, local networks blocked
	pol := migrationTestPolicy("github.com", "private, loopback")
	assert.Error(t, isMigrateURLAllowed(pol, "https://gitlab.com/gitlab/gitlab.git", nonAdminUser))
	assert.NoError(t, isMigrateURLAllowed(pol, "https://github.com/go-gitea/gitea.git", nonAdminUser))
	assert.NoError(t, isMigrateURLAllowed(pol, "https://gITHUb.com/go-gitea/gitea.git", nonAdminUser))

	// BLOCKED_DOMAINS=github.com, local networks still blocked
	pol = migrationTestPolicy("external", "github.com, private, loopback")
	assert.NoError(t, isMigrateURLAllowed(pol, "https://gitlab.com/gitlab/gitlab.git", nonAdminUser))
	assert.Error(t, isMigrateURLAllowed(pol, "https://github.com/go-gitea/gitea.git", nonAdminUser))
	assert.Error(t, isMigrateURLAllowed(pol, "https://10.0.0.1/go-gitea/gitea.git", nonAdminUser))

	// ALLOW_LOCALNETWORKS=true lifts the local network block
	pol = migrationTestPolicy("external, private, loopback", "github.com")
	assert.NoError(t, isMigrateURLAllowed(pol, "https://10.0.0.1/go-gitea/gitea.git", nonAdminUser))

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = false

	assert.Error(t, isMigrateURLAllowed(pol, "/home/foo/bar/goo", adminUser))

	setting.ImportLocalPaths = true
	abs, err := filepath.Abs(".")
	assert.NoError(t, err)

	assert.NoError(t, isMigrateURLAllowed(pol, abs, adminUser))
	assert.Error(t, isMigrateURLAllowed(pol, abs, nonAdminUser))

	nonAdminUser.AllowImportLocal = true
	assert.NoError(t, isMigrateURLAllowed(pol, abs, nonAdminUser))

	setting.ImportLocalPaths = old
}

func TestAllowBlockList(t *testing.T) {
	// default, allow all external, block none, no local networks
	pol := migrationTestPolicy("external", "private, loopback")
	assert.NoError(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// allow all including local networks (it could lead to SSRF in production)
	pol = migrationTestPolicy("external, private, loopback", "")
	assert.NoError(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.NoError(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// allow wildcard, block some subdomains. every resolved address must still be allowed.
	pol = migrationTestPolicy("*.domain.com", "blocked.domain.com, private, loopback")
	assert.NoError(t, checkByAllowBlockList(pol, "sub.domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList(pol, "sub.domain.com", []net.IP{net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList(pol, "sub.domain.com", []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList(pol, "blocked.domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList(pol, "sub.other.com", []net.IP{net.ParseIP("1.2.3.4")}))

	// allow wildcard still follows the local network policy for resolved addresses.
	pol = migrationTestPolicy("*", "private, loopback")
	assert.NoError(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("127.0.0.1")}))

	// local network can still be blocked explicitly
	pol = migrationTestPolicy("*", "127.0.0.*")
	assert.NoError(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList(pol, "domain.com", []net.IP{net.ParseIP("127.0.0.1")}))
}
