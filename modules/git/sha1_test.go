// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidSHAPattern(t *testing.T) {
	assert.True(t, IsValidSHAPattern("fee1"))
	assert.True(t, IsValidSHAPattern("abc000"))
	assert.True(t, IsValidSHAPattern("9023902390239023902390239023902390239023"))
	assert.False(t, IsValidSHAPattern("90239023902390239023902390239023902390239023"))
	assert.False(t, IsValidSHAPattern("abc"))
	assert.False(t, IsValidSHAPattern("123g"))
	assert.False(t, IsValidSHAPattern("some random text"))
}
