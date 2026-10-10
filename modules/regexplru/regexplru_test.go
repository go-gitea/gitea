// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package regexplru

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegexpLru(t *testing.T) {
	r, err := UserCache().GetCompiled("a")
	assert.NoError(t, err)
	assert.True(t, r.MatchString("a"))

	r, err = UserCache().GetCompiled("a")
	assert.NoError(t, err)
	assert.True(t, r.MatchString("a"))
	assert.Equal(t, 1, UserCache().lruCache.Len())

	_, err = UserCache().GetCompiled("(")
	assert.Error(t, err)
	assert.Equal(t, 2, UserCache().lruCache.Len())
}
