// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessTokenScope_Normalize(t *testing.T) {
	tests := []struct {
		in  AccessTokenScope
		out AccessTokenScope
		err error
	}{
		{"", "", nil},
		{"repo", "repo", nil},
		{"repo,repo:status", "repo", nil},
		{"repo,public_repo", "repo", nil},
		{"admin:public_key,write:public_key", "admin:public_key", nil},
		{"admin:public_key,read:public_key", "admin:public_key", nil},
		{"admin:repo_hook,write:repo_hook", "admin:repo_hook", nil},
		{"admin:repo_hook,read:repo_hook", "admin:repo_hook", nil},
		{"repo,admin:repo_hook,read:repo_hook", "repo", nil}, // admin:repo_hook is a child scope of repo
		{"repo,read:repo_hook", "repo", nil},                 // read:repo_hook is a child scope of repo
		{"user", "user", nil},
		{"user,read:user", "user", nil},
		{"user,admin:org,write:org", "admin:org,user", nil},
		{"admin:org,write:org,user", "admin:org,user", nil},
		{"package", "package", nil},
		{"package,write:package", "package", nil},
		{"package,write:package,delete:package", "package", nil},
		{"admin:gpg_key", "admin:gpg_key", nil},
		{"admin:gpg_key,write:gpg_key", "admin:gpg_key", nil},
		{"admin:gpg_key,write:gpg_key,user", "user,admin:gpg_key", nil},
		{"admin:application,write:application,user", "user,admin:application", nil},
		{"all", "all", nil},
		{"repo,admin:org,admin:public_key,admin:repo_hook,admin:org_hook,notification,user,delete_repo,package,admin:gpg_key,admin:application", "all", nil},
		{"repo,admin:org,admin:public_key,admin:repo_hook,admin:org_hook,notification,user,delete_repo,package,admin:gpg_key,admin:application,sudo", "all,sudo", nil},
	}

	for _, test := range tests {
		t.Run(string(test.in), func(t *testing.T) {
			scope, err := test.in.Normalize()
			assert.Equal(t, test.out, scope)
			assert.Equal(t, test.err, err)
		})
	}
}

func TestAccessTokenScope_HasScope(t *testing.T) {
	tests := []struct {
		in    AccessTokenScope
		scope string
		out   bool
		err   error
	}{
		{"repo", "repo", true, nil},
		{"repo", "repo:status", true, nil},
		{"repo", "public_repo", true, nil},
		{"repo", "admin:org", false, nil},
		{"repo", "admin:public_key", false, nil},
		{"repo:status", "repo", false, nil},
		{"repo:status", "public_repo", false, nil},
		{"admin:org", "write:org", true, nil},
		{"admin:org", "read:org", true, nil},
		{"admin:org", "admin:org", true, nil},
		{"user", "read:user", true, nil},
		{"package", "write:package", true, nil},
	}

	for _, test := range tests {
		t.Run(string(test.in), func(t *testing.T) {
			scope, err := test.in.HasScope(test.scope)
			assert.Equal(t, test.out, scope)
			assert.Equal(t, test.err, err)
		})
	}
}
