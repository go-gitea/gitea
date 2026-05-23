// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMapValueOrDefault(t *testing.T) {
	testMap := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": nil,
	}

	assert.Equal(t, "value1", GetMapValueOrDefault(testMap, "key1", "default"))
	assert.Equal(t, 42, GetMapValueOrDefault(testMap, "key2", 0))

	assert.Equal(t, "default", GetMapValueOrDefault(testMap, "key4", "default"))
	assert.Equal(t, 100, GetMapValueOrDefault(testMap, "key5", 100))

	assert.Equal(t, "default", GetMapValueOrDefault(testMap, "key3", "default"))
}
