// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package db

package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidUsername(t *testing.T) {
	assert.True(t, IsValidUsername("a"))
	assert.True(t, IsValidUsername("abc"))
	assert.True(t, IsValidUsername("0.b-c"))
	assert.True(t, IsValidUsername("a.b-c_d"))

	assert.False(t, IsValidUsername(""))
	assert.False(t, IsValidUsername(".abc"))
	assert.False(t, IsValidUsername("abc."))
	assert.False(t, IsValidUsername("a..bc"))
	assert.False(t, IsValidUsername("a...bc"))
	assert.False(t, IsValidUsername("a.-bc"))
	assert.False(t, IsValidUsername("a._bc"))
	assert.False(t, IsValidUsername("a_-bc"))
	assert.False(t, IsValidUsername("a/bc"))
	assert.False(t, IsValidUsername("☁️"))
}
