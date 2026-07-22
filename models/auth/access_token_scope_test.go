// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type scopeTestNormalize struct {
	in  AccessTokenScope
	out AccessTokenScope
	err error
}

func TestAccessTokenScope_Normalize(t *testing.T) {
	assert.Equal(t, []string{"activitypub", "admin", "issue", "misc", "notification", "organization", "package", "repository", "user"}, GetAccessTokenCategories())
	tests := []scopeTestNormalize{
		{"", "", nil},
		{"write:misc,write:notification,read:package,write:notification,public-only", "public-only,write:misc,write:notification,read:package", nil},
		{"all", "all", nil},
		{"write:activitypub,write:admin,write:misc,write:notification,write:organization,write:package,write:issue,write:repository,write:user", "all", nil},
		{"write:activitypub,write:admin,write:misc,write:notification,write:organization,write:package,write:issue,write:repository,write:user,public-only", "public-only,all", nil},
	}

	for _, scope := range GetAccessTokenCategories() {
		tests = append(tests,
			scopeTestNormalize{AccessTokenScope("read:" + scope), AccessTokenScope("read:" + scope), nil},
			scopeTestNormalize{AccessTokenScope("write:" + scope), AccessTokenScope("write:" + scope), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("write:%[1]s,read:%[1]s", scope)), AccessTokenScope("write:" + scope), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("read:%[1]s,write:%[1]s", scope)), AccessTokenScope("write:" + scope), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("read:%[1]s,write:%[1]s,write:%[1]s", scope)), AccessTokenScope("write:" + scope), nil},
		)
	}

	for _, test := range tests {
		t.Run(string(test.in), func(t *testing.T) {
			scope, err := test.in.Normalize()
			assert.Equal(t, test.out, scope)
			assert.Equal(t, test.err, err)
		})
	}
}

type scopeTestHasScope struct {
	in    AccessTokenScope
	scope AccessTokenScope
	out   bool
	err   error
}

func TestAccessTokenScope_HasScope(t *testing.T) {
	tests := []scopeTestHasScope{
		{"read:admin", "write:package", false, nil},
		{"all", "write:package", true, nil},
		{"write:package", "all", false, nil},
		{"public-only", "read:issue", false, nil},
	}

	for _, scope := range GetAccessTokenCategories() {
		tests = append(tests,
			scopeTestHasScope{
				AccessTokenScope("read:" + scope),
				AccessTokenScope("read:" + scope), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope("write:" + scope),
				AccessTokenScope("write:" + scope), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope("write:" + scope),
				AccessTokenScope("read:" + scope), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope("read:" + scope),
				AccessTokenScope("write:" + scope), false, nil,
			},
		)
	}

	for _, test := range tests {
		t.Run(string(test.in), func(t *testing.T) {
			hasScope, err := test.in.HasScope(test.scope)
			assert.Equal(t, test.out, hasScope)
			assert.Equal(t, test.err, err)
		})
	}
}

func TestAccessTokenScope_EnforcePublicOnlyFrom(t *testing.T) {
	tests := []struct {
		in     AccessTokenScope
		parent AccessTokenScope
		out    AccessTokenScope
	}{
		// public-only parent forces the restriction onto the minted scope
		{"write:user", "write:user,public-only", "public-only,write:user"},
		// already public-only stays public-only
		{"public-only,read:user", "public-only", "public-only,read:user"},
		// non-public-only parent leaves the scope untouched
		{"write:user", "write:user", "write:user"},
		{"all", "all", "all"},
	}
	for _, test := range tests {
		t.Run(string(test.parent)+"->"+string(test.in), func(t *testing.T) {
			got, err := test.in.EnforcePublicOnlyFrom(test.parent)
			assert.NoError(t, err)
			assert.Equal(t, test.out, got)
		})
	}
}
