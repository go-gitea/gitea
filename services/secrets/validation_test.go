// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name        string
		valid       bool
		errorContains string // substring that should appear in error message
	}{
		{"FOO", true, ""},
		{"FOO1_BAR2", true, ""},
		{"_FOO", true, ""},
		{"1FOO", false, "start with a letter or underscore"},
		{"giteA_xx", false, "GITEA_"},
		{"githuB_xx", false, "GITHUB_"},
		{"cI", false, "reserved"},
	}
	for _, c := range cases {
		err := ValidateName(c.name)
		if c.valid {
			assert.NoError(t, err, "ValidateName(%q) should be valid", c.name)
		} else {
			assert.Error(t, err, "ValidateName(%q) should be invalid", c.name)
			if c.errorContains != "" {
				assert.Contains(t, err.Error(), c.errorContains, "ValidateName(%q) error message should mention %q", c.name, c.errorContains)
			}
		}
	}
}
