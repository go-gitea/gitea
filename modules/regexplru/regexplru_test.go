// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package regexplru

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegexpLru(t *testing.T) {
	r, err := GetCompiled("a")
	assert.NoError(t, err)
	assert.True(t, r.MatchString("a"))

	r, err = GetCompiled("a")
	assert.NoError(t, err)
	assert.True(t, r.MatchString("a"))

	assert.EqualValues(t, 1, lruCache.Len())

	_, err = GetCompiled("(")
	assert.Error(t, err)
	assert.EqualValues(t, 2, lruCache.Len())
}
