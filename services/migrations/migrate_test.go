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
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/assert"
)

type testStderrError struct{ stderr string }

func (err testStderrError) Error() string  { return err.stderr }
func (err testStderrError) Unwrap() error  { return nil }
func (err testStderrError) Stderr() string { return err.stderr }

func TestIsAuthenticationError(t *testing.T) {
	githubError := func(code int) error {
		return &github.ErrorResponse{Response: &http.Response{StatusCode: code}}
	}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"git authentication failed", testStderrError{string(gitcmd.StderrAuthenticationFailed) + " 'https://host/repo.git/'"}, true},
		{"git could not read username", testStderrError{string(gitcmd.StderrCouldNotReadUsername) + " for 'https://host'"}, true},
		{"github unauthorized", githubError(http.StatusUnauthorized), true},
		{"github unauthorized sanitized", util.SanitizeErrorCredentialURLs(githubError(http.StatusUnauthorized)), true},
		{"github unauthorized wrapped", fmt.Errorf("migrate: %w", githubError(http.StatusUnauthorized)), true},
		{"github not found", githubError(http.StatusNotFound), false},
		{"github without response", &github.ErrorResponse{}, false},
		{"unrelated error", errors.New("something else"), false},
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

	// allow wildcard, block some subdomains. every resolved address must still be allowed.
	init("*.domain.com", "blocked.domain.com", false)
	assert.NoError(t, checkByAllowBlockList("sub.domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("sub.domain.com", []net.IP{net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList("sub.domain.com", []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList("blocked.domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("sub.other.com", []net.IP{net.ParseIP("1.2.3.4")}))

	// allow wildcard still follows the local network policy for resolved addresses.
	init("*", "", false)
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("127.0.0.1")}))
	assert.Error(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("127.0.0.1")}))

	// local network can still be blocked
	init("*", "127.0.0.*", false)
	assert.NoError(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("1.2.3.4")}))
	assert.Error(t, checkByAllowBlockList("domain.com", []net.IP{net.ParseIP("127.0.0.1")}))

	// reset to allow local networks (mock servers use 127.0.0.1)
	init("", "", true)
}
