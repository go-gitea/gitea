// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/structs"

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

func TestLoadServiceVisibilityModes(t *testing.T) {
	oldService := Service
	defer func() {
		Service = oldService
	}()

	kases := map[string]func(){
		`
[service]
DEFAULT_USER_VISIBILITY = public
ALLOWED_USER_VISIBILITY_MODES = public,limited,private
`: func() {
			assert.Equal(t, "public", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypePublic, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"public", "limited", "private"}, Service.AllowedUserVisibilityModes)
		},
		`
		[service]
		DEFAULT_USER_VISIBILITY = public
		`: func() {
			assert.Equal(t, "public", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypePublic, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"public", "limited", "private"}, Service.AllowedUserVisibilityModes)
		},
		`
		[service]
		DEFAULT_USER_VISIBILITY = limited
		`: func() {
			assert.Equal(t, "limited", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypeLimited, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"public", "limited", "private"}, Service.AllowedUserVisibilityModes)
		},
		`
[service]
ALLOWED_USER_VISIBILITY_MODES = public,limited,private
`: func() {
			assert.Equal(t, "public", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypePublic, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"public", "limited", "private"}, Service.AllowedUserVisibilityModes)
		},
		`
[service]
DEFAULT_USER_VISIBILITY = public
ALLOWED_USER_VISIBILITY_MODES = limited,private
`: func() {
			assert.Equal(t, "limited", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypeLimited, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"limited", "private"}, Service.AllowedUserVisibilityModes)
		},
		`
[service]
DEFAULT_USER_VISIBILITY = my_type
ALLOWED_USER_VISIBILITY_MODES = limited,private
`: func() {
			assert.Equal(t, "limited", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypeLimited, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"limited", "private"}, Service.AllowedUserVisibilityModes)
		},
		`
[service]
DEFAULT_USER_VISIBILITY = public
ALLOWED_USER_VISIBILITY_MODES = public, limit, privated
`: func() {
			assert.Equal(t, "public", Service.DefaultUserVisibility)
			assert.Equal(t, structs.VisibleTypePublic, Service.DefaultUserVisibilityMode)
			assert.Equal(t, []string{"public"}, Service.AllowedUserVisibilityModes)
		},
	}

	for kase, fun := range kases {
		t.Run(kase, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(kase)
			assert.NoError(t, err)
			loadServiceFrom(cfg)
			fun()
			// reset
			Service.AllowedUserVisibilityModesSlice = []bool{true, true, true}
			Service.AllowedUserVisibilityModes = []string{}
			Service.DefaultUserVisibility = ""
			Service.DefaultUserVisibilityMode = structs.VisibleTypePublic
		})
	}
}
