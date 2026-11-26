// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeOIDCScopes(t *testing.T) {
	for _, tc := range []struct {
		name           string
		global, source []string
		expected       []string
	}{
		{
			name:     "no scopes",
			expected: nil,
		},
		{
			name:     "source only",
			source:   []string{"custom_scope"},
			expected: []string{"custom_scope"},
		},
		{
			name:     "global only",
			global:   []string{"openid", "email"},
			expected: []string{"openid", "email"},
		},
		{
			name:     "global first then new source scopes",
			global:   []string{"openid", "email"},
			source:   []string{"custom_scope", "groups"},
			expected: []string{"openid", "email", "custom_scope", "groups"},
		},
		{
			name:     "duplicates are dropped",
			global:   []string{"openid", "email"},
			source:   []string{"email", "custom_scope", "openid"},
			expected: []string{"openid", "email", "custom_scope"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			global := append([]string(nil), tc.global...)
			assert.Equal(t, tc.expected, mergeOIDCScopes(global, tc.source))
			assert.Equal(t, tc.global, global, "the shared config scopes must not be modified")
		})
	}
}
