// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
)

func TestLoadServices(t *testing.T) {
	oldService := Service
	defer func() {
		Service = oldService
	}()

	cfg, err := NewConfigProviderFromData(`
[service]
EMAIL_DOMAIN_WHITELIST = d1, *.w
EMAIL_DOMAIN_ALLOWLIST = d2, *.a
EMAIL_DOMAIN_BLOCKLIST = d3, *.b
`)
	assert.NoError(t, err)
	loadServiceFrom(cfg)

	match := func(globs []glob.Glob, s string) bool {
		for _, g := range globs {
			if g.Match(s) {
				return true
			}
		}
		return false
	}

	assert.True(t, match(Service.EmailDomainAllowList, "d1"))
	assert.True(t, match(Service.EmailDomainAllowList, "foo.w"))
	assert.True(t, match(Service.EmailDomainAllowList, "d2"))
	assert.True(t, match(Service.EmailDomainAllowList, "foo.a"))
	assert.False(t, match(Service.EmailDomainAllowList, "d3"))

	assert.True(t, match(Service.EmailDomainBlockList, "d3"))
	assert.True(t, match(Service.EmailDomainBlockList, "foo.b"))
	assert.False(t, match(Service.EmailDomainBlockList, "d1"))
}
