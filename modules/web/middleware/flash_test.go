// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlashMessageEncode(t *testing.T) {
	assert.Empty(t, flashMessageEncode(nil, 100))

	messages := map[string]string{"error": "<div>\n<div>", "success": "🌞"}
	assert.Equal(t, messages, flashMessageDecode(flashMessageEncode(messages, 100)))
	assert.Equal(t, map[string]string{"error": "删除分支失败"}, flashMessageDecode(flashMessageEncode(map[string]string{"error": "删除分支\xff失败"}, 100)))

	const limit = 99
	encoded := flashMessageEncode(map[string]string{"error": strings.Repeat("x", 200)}, limit)
	assert.Len(t, encoded, limit/3*4)
	decoded := flashMessageDecode(encoded)
	assert.Equal(t, map[string]string{"error": strings.Repeat("x", limit-len("error="))}, decoded)

	// also support legacy format
	decoded = flashMessageDecode("error=invalid%20message")
	assert.Equal(t, map[string]string{"error": "invalid message"}, decoded)
}
