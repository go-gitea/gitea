// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package generic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateNames(t *testing.T) {
	for _, test := range []struct {
		name      string
		validate  func(string) bool
		bad, good []string
	}{
		{
			name:     "package",
			validate: isValidPackageName,
			bad:      []string{"", ".", "..", "-", "a?b", "a b", "a/b"},
			good:     []string{"a", "1", "a-", "a_b", "c.d+"},
		},
		{
			name:     "file",
			validate: isValidFileName,
			bad:      []string{"", ".", "..", "a?b", "a/b", " a", "a "},
			good:     []string{"-", "a", "1", "a-", "a_b", "a b", "c.d+", `-_+=:;.()[]{}~!@#$%^& aA1`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, name := range test.bad {
				assert.False(t, test.validate(name), "bad=%q", name)
			}
			for _, name := range test.good {
				assert.True(t, test.validate(name), "good=%q", name)
			}
		})
	}
}
