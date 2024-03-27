// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package generic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePackageName(t *testing.T) {
	bad := []string{
		"",
		".",
		"..",
		"-",
		"a?b",
		"a b",
		"a/b",
	}
	for _, name := range bad {
		assert.False(t, isValidPackageName(name), "bad=%q", name)
	}

	good := []string{
		"a",
		"1",
		"a-",
		"a_b",
		"c.d+",
	}
	for _, name := range good {
		assert.True(t, isValidPackageName(name), "good=%q", name)
	}
}

func TestValidateFileName(t *testing.T) {
	bad := []string{
		"",
		".",
		"..",
		"a?b",
		"a/b",
		" a",
		"a ",
	}
	for _, name := range bad {
		assert.False(t, isValidFileName(name), "bad=%q", name)
	}

	good := []string{
		"-",
		"a",
		"1",
		"a-",
		"a_b",
		"a b",
		"c.d+",
		`-_+=:;.()[]{}~!@#$%^& aA1`,
	}
	for _, name := range good {
		assert.True(t, isValidFileName(name), "good=%q", name)
	}
}
