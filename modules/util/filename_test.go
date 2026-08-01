// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileNameJoinFields(t *testing.T) {
	assert.Equal(t, "a-1.txt", FileNameJoinFields("a", 1, ".txt"))
	assert.Equal(t, "🌞-🌛.txt", fileNameJoinFields(14, "🌞", "🌛", ".txt"))
	assert.Equal(t, "🌞-🌛__.txt", fileNameJoinFields(14, "🌞", "🌛🌛🌛🌛", ".txt"))
	assert.Equal(t, "🌞__-__.txt", fileNameJoinFields(14, "🌞🌞🌞🌞", "🌛🌛🌛🌛", ".txt"))
}
