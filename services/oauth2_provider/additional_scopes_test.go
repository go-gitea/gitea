// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_provider //nolint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrantAdditionalScopes(t *testing.T) {
	tests := []struct {
		grantScopes    string
		expectedScopes string
	}{
		{"openid profile email", "all"},
		{"openid profile email groups", "all"},
		{"openid profile email all", "all"},
		{"openid profile email read:user all", "all"},
		{"openid profile email groups read:user", "read:user"},
		{"read:user read:repository", "read:repository,read:user"},
		{"read:user write:issue public-only", "public-only,write:issue,read:user"},
		{"openid profile email read:user", "read:user"},
		{"read:invalid_scope", "all"},
		{"read:invalid_scope,write:scope_invalid,just-plain-wrong", "all"},
	}

	for _, test := range tests {
		t.Run(test.grantScopes, func(t *testing.T) {
			result := GrantAdditionalScopes(test.grantScopes)
			assert.Equal(t, test.expectedScopes, string(result))
		})
	}
}
