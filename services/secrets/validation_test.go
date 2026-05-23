// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"FOO", true},
		{"FOO1_BAR2", true},
		{"_FOO", true}, // really? why support this
		{"1FOO", false},
		{"giteA_xx", false},
		{"githuB_xx", false},
		{"cI", false},
	}
	for _, c := range cases {
		err := ValidateName(c.name)
		assert.Equal(t, c.valid, err == nil, "ValidateName(%q)", c.name)
	}
}
