// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatByteSize(t *testing.T) {
	var size int64 = 512
	assert.Equal(t, "512 B", FormatByteSize(size))
	size *= 1024
	assert.Equal(t, "512 KiB", FormatByteSize(size))
	size *= 1024
	assert.Equal(t, "512 MiB", FormatByteSize(size))
	size *= 1024
	assert.Equal(t, "512 GiB", FormatByteSize(size))
	size *= 1024
	assert.Equal(t, "512 TiB", FormatByteSize(size))
	size *= 1024
	assert.Equal(t, "512 PiB", FormatByteSize(size))
	size *= 4
	assert.Equal(t, "2.0 EiB", FormatByteSize(size))
}
