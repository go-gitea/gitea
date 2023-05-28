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
	tests := []scopeTestNormalize{
		{"", "", nil},
		{"write:misc,delete:notification,read:package,write:notification,public-only", "public-only,write:misc,delete:notification,read:package", nil},
		{"all", "all", nil},
		{"delete:activitypub,delete:admin,delete:misc,delete:notification,delete:organization,delete:package,delete:issue,delete:repository,delete:user", "all", nil},
		{"delete:activitypub,delete:admin,delete:misc,delete:notification,delete:organization,delete:package,delete:issue,delete:repository,delete:user,public-only", "public-only,all", nil},
	}

	for _, scope := range []string{"activitypub", "admin", "misc", "notification", "organization", "package", "issue", "repository", "user"} {
		tests = append(tests,
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("read:%s", scope)), AccessTokenScope(fmt.Sprintf("read:%s", scope)), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("write:%s", scope)), AccessTokenScope(fmt.Sprintf("write:%s", scope)), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("write:%[1]s,read:%[1]s", scope)), AccessTokenScope(fmt.Sprintf("write:%s", scope)), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("read:%[1]s,write:%[1]s", scope)), AccessTokenScope(fmt.Sprintf("write:%s", scope)), nil},
			scopeTestNormalize{AccessTokenScope(fmt.Sprintf("read:%[1]s,delete:%[1]s,write:%[1]s", scope)), AccessTokenScope(fmt.Sprintf("delete:%s", scope)), nil},
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
		{"read:admin", "delete:package", false, nil},
		{"all", "delete:package", true, nil},
		{"delete:package", "all", false, nil},
		{"public-only", "read:issue", false, nil},
	}

	for _, scope := range []string{"activitypub", "admin", "misc", "notification", "organization", "package", "issue", "repository", "user"} {
		tests = append(tests,
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("read:%s", scope)),
				AccessTokenScope(fmt.Sprintf("read:%s", scope)), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("write:%s", scope)),
				AccessTokenScope(fmt.Sprintf("write:%s", scope)), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("delete:%s", scope)),
				AccessTokenScope(fmt.Sprintf("delete:%s", scope)), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("write:%s", scope)),
				AccessTokenScope(fmt.Sprintf("read:%s", scope)), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("delete:%s", scope)),
				AccessTokenScope(fmt.Sprintf("read:%s", scope)), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("delete:%s", scope)),
				AccessTokenScope(fmt.Sprintf("write:%s", scope)), true, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("read:%s", scope)),
				AccessTokenScope(fmt.Sprintf("write:%s", scope)), false, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("read:%s", scope)),
				AccessTokenScope(fmt.Sprintf("delete:%s", scope)), false, nil,
			},
			scopeTestHasScope{
				AccessTokenScope(fmt.Sprintf("write:%s", scope)),
				AccessTokenScope(fmt.Sprintf("delete:%s", scope)), false, nil,
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
