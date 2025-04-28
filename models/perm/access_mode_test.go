// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package perm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessMode(t *testing.T) {
	names := []string{"none", "read", "write", "admin"}
	for i, name := range names {
		m := ParseAccessMode(name)
		assert.Equal(t, AccessMode(i), m)
	}
	assert.Equal(t, AccessModeOwner, AccessMode(4))
	assert.Equal(t, "owner", AccessModeOwner.ToString())
	assert.Equal(t, AccessModeNone, ParseAccessMode("owner"))
	assert.Equal(t, AccessModeNone, ParseAccessMode("invalid"))
}
