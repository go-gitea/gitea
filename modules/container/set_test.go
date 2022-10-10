// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	s := make(Set[string])

	assert.True(t, s.Add("key1"))
	assert.False(t, s.Add("key1"))
	assert.True(t, s.Add("key2"))

	assert.True(t, s.Contains("key1"))
	assert.True(t, s.Contains("key2"))
	assert.False(t, s.Contains("key3"))

	assert.True(t, s.Remove("key2"))
	assert.False(t, s.Contains("key2"))

	assert.False(t, s.Remove("key3"))
}
