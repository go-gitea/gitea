// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name   string
		errMsg string // substring that should appear in error message
	}{
		{"FOO", ""},
		{"FOO1_BAR2", ""},
		{"_Foo", ""},

		{"FOO.BAR", "contain only letters, numbers, and underscores"},
		{"1FOO", "name must start with a letter or underscore"},
		{"giteA_xx", "name cannot start with"},
		{"githuB_xx", "name cannot start with"},
		{"cI", "is a reserved name"},
	}
	for _, c := range cases {
		err := ValidateName(c.name)
		if c.errMsg == "" {
			assert.NoError(t, err, "ValidateName(%q) should be valid", c.name)
		} else {
			assert.ErrorContains(t, err, c.errMsg, "ValidateName(%q) error message should mention %q", c.name, c.errMsg)
		}
	}
}
