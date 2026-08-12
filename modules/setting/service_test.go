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
	allowedModes := func() []structs.VisibleType { return Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice() }
	allVisible := visibleTypeSlice("public", "limited", "private")

	testConfigLoad(t, []any{loadServiceFrom}, []configTestCase{
		{
			ini: "[service]\nDEFAULT_USER_VISIBILITY = public\nALLOWED_USER_VISIBILITY_MODES = public,limited,private",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypePublic),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, allVisible),
			},
		},
		{
			ini: "[service]\nDEFAULT_USER_VISIBILITY = public",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypePublic),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, allVisible),
			},
		},
		{
			ini: "[service]\nDEFAULT_USER_VISIBILITY = limited",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypeLimited),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, allVisible),
			},
		},
		{
			ini: "[service]\nALLOWED_USER_VISIBILITY_MODES = public,limited,private",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypePublic),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, allVisible),
			},
		},
		{
			name: "the default falls back to the first allowed mode",
			ini:  "[service]\nDEFAULT_USER_VISIBILITY = public\nALLOWED_USER_VISIBILITY_MODES = limited,private",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypeLimited),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, visibleTypeSlice("limited", "private")),
			},
		},
		{
			name: "an unknown default falls back to the first allowed mode",
			ini:  "[service]\nDEFAULT_USER_VISIBILITY = my_type\nALLOWED_USER_VISIBILITY_MODES = limited,private",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypeLimited),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, visibleTypeSlice("limited", "private")),
			},
		},
		{
			name: "an unknown default alone falls back to public",
			ini:  "[service]\nDEFAULT_USER_VISIBILITY = my_type",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypePublic),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, allVisible),
			},
		},
		{
			name: "unknown allowed modes are dropped",
			ini:  "[service]\nDEFAULT_USER_VISIBILITY = public\nALLOWED_USER_VISIBILITY_MODES = public, limit, privated",
			want: []configCheck{
				field("DEFAULT_USER_VISIBILITY", &Service.DefaultUserVisibilityMode, structs.VisibleTypePublic),
				fieldOf("ALLOWED_USER_VISIBILITY_MODES", allowedModes, visibleTypeSlice("public")),
			},
		},
	})
}

func TestLoadServiceRequireSignInView(t *testing.T) {
	defer test.MockVariableValue(&Service)()

	testConfigLoad(t, []any{loadServiceFrom}, []configTestCase{
		{
			name: "unset",
			ini:  "[service]",
			want: []configCheck{
				field("REQUIRE_SIGNIN_VIEW", &Service.RequireSignInViewStrict, false),
				field("REQUIRE_SIGNIN_VIEW", &Service.BlockAnonymousAccessExpensive, false),
			},
		},
		{
			name: "true is strict",
			ini:  "[service]\nREQUIRE_SIGNIN_VIEW = true",
			want: []configCheck{
				field("REQUIRE_SIGNIN_VIEW", &Service.RequireSignInViewStrict, true),
				field("REQUIRE_SIGNIN_VIEW", &Service.BlockAnonymousAccessExpensive, false),
			},
		},
		{
			name: "expensive blocks anonymous access instead",
			ini:  "[service]\nREQUIRE_SIGNIN_VIEW = expensive",
			want: []configCheck{
				field("REQUIRE_SIGNIN_VIEW", &Service.RequireSignInViewStrict, false),
				field("REQUIRE_SIGNIN_VIEW", &Service.BlockAnonymousAccessExpensive, true),
			},
		},
	})
}
