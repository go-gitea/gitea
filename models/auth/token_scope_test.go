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
		{"all", "all", nil},
		{"repo,admin:org,admin:public_key,admin:repo_hook,admin:org_hook,notification,user,delete_repo,package,admin:gpg_key", "all", nil},
	}

	for _, test := range tests {
		t.Run(string(test.in), func(t *testing.T) {
			scope, err := test.in.Normalize()
			assert.Equal(t, test.out, scope)
			assert.Equal(t, test.err, err)
		})
	}
}
