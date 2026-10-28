// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/glob"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadServices(t *testing.T) {
	defer test.MockVariableValue(&Service)()

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
	defer test.MockVariableValue(&Service)()

	visibleTypeSlice := func(s ...structs.VisibilityString) (ret []structs.VisibleType) {
		for _, v := range s {
			ret = append(ret, structs.VisibilityModes[v])
		}
		return ret
	}
	testCases := map[string]struct {
		defaultMode  structs.VisibleType
		allowedModes []structs.VisibilityString
	}{
		`
[service]
DEFAULT_USER_VISIBILITY = public
ALLOWED_USER_VISIBILITY_MODES = public,limited,private
`: {structs.VisibleTypePublic, []structs.VisibilityString{"public", "limited", "private"}},
		`
		[service]
		DEFAULT_USER_VISIBILITY = public
		`: {structs.VisibleTypePublic, []structs.VisibilityString{"public", "limited", "private"}},
		`
		[service]
		DEFAULT_USER_VISIBILITY = limited
		`: {structs.VisibleTypeLimited, []structs.VisibilityString{"public", "limited", "private"}},
		`
[service]
ALLOWED_USER_VISIBILITY_MODES = public,limited,private
`: {structs.VisibleTypePublic, []structs.VisibilityString{"public", "limited", "private"}},
		`
[service]
DEFAULT_USER_VISIBILITY = public
ALLOWED_USER_VISIBILITY_MODES = limited,private
`: {structs.VisibleTypeLimited, []structs.VisibilityString{"limited", "private"}},
		`
[service]
DEFAULT_USER_VISIBILITY = my_type
ALLOWED_USER_VISIBILITY_MODES = limited,private
`: {structs.VisibleTypeLimited, []structs.VisibilityString{"limited", "private"}},
		`
[service]
DEFAULT_USER_VISIBILITY = my_type
`: {structs.VisibleTypePublic, []structs.VisibilityString{"public", "limited", "private"}},
		`
[service]
DEFAULT_USER_VISIBILITY = public
ALLOWED_USER_VISIBILITY_MODES = public, limit, privated
`: {structs.VisibleTypePublic, []structs.VisibilityString{"public"}},
	}

	for tc, expected := range testCases {
		t.Run(tc, func(t *testing.T) {
			Service.AllowedUserVisibilityModesSlice = []bool{true, true, true}
			Service.DefaultUserVisibilityMode = structs.VisibleTypePublic
			cfg, err := NewConfigProviderFromData(tc)
			assert.NoError(t, err)
			loadServiceFrom(cfg)
			assert.Equal(t, expected.defaultMode, Service.DefaultUserVisibilityMode)
			assert.Equal(t, visibleTypeSlice(expected.allowedModes...), Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice())
		})
	}
}

func TestLoadServiceRequireSignInView(t *testing.T) {
	defer test.MockVariableValue(&Service)()

	cfg, err := NewConfigProviderFromData(`
[service]
`)
	assert.NoError(t, err)
	loadServiceFrom(cfg)
	assert.False(t, Service.RequireSignInViewStrict)
	assert.False(t, Service.BlockAnonymousAccessExpensive)

	cfg, err = NewConfigProviderFromData(`
[service]
REQUIRE_SIGNIN_VIEW = true
`)
	assert.NoError(t, err)
	loadServiceFrom(cfg)
	assert.True(t, Service.RequireSignInViewStrict)
	assert.False(t, Service.BlockAnonymousAccessExpensive)

	cfg, err = NewConfigProviderFromData(`
[service]
REQUIRE_SIGNIN_VIEW = expensive
`)
	assert.NoError(t, err)
	loadServiceFrom(cfg)
	assert.False(t, Service.RequireSignInViewStrict)
	assert.True(t, Service.BlockAnonymousAccessExpensive)
}
