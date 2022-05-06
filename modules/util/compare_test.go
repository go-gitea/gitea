// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringMatchesPattern(t *testing.T) {
	// Make sure non-wildcard matching works
	assert.True(t, StringMatchesPattern("fubar", "fubar"))

	// Make sure wildcard matching accepts
	assert.True(t, StringMatchesPattern("A is not B", "A*B"))
	assert.True(t, StringMatchesPattern("A is not B", "A*"))

	// Make sure wildcard matching rejects
	assert.False(t, StringMatchesPattern("fubar", "A*B"))
	assert.False(t, StringMatchesPattern("A is not b", "A*B"))

	// Make sure regexp specials are escaped
	assert.False(t, StringMatchesPattern("A is not B", "[aA]*"))
	assert.True(t, StringMatchesPattern("[aA] is not B", "[aA]*"))
}
