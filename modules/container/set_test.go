// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	assert.True(t, s.Contains("key1", "key2"))
	assert.False(t, s.Contains("key3"))
	assert.False(t, s.Contains("key1", "key3"))

	assert.True(t, s.Remove("key2"))
	assert.False(t, s.Contains("key2"))

	assert.False(t, s.Remove("key3"))

	s.AddMultiple("key4", "key5")
	assert.True(t, s.Contains("key4"))
	assert.True(t, s.Contains("key5"))

	s = SetOf("key6", "key7")
	assert.False(t, s.Contains("key1"))
	assert.True(t, s.Contains("key6"))
	assert.True(t, s.Contains("key7"))
}
