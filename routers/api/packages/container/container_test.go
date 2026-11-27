// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageNamePattern(t *testing.T) {
	for _, name := range []string{"a", "a.b", "a_b", "a__b", "a-b", "a--b", "a---b", "sub/a__b/c--d"} {
		assert.True(t, globalVars().imageNamePattern.MatchString(name), name)
	}
	for _, name := range []string{"", "A", "a___b", "a._b", "-a", "a-", "a//b", "/a", "a/"} {
		assert.False(t, globalVars().imageNamePattern.MatchString(name), name)
	}
}
