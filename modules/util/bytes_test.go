// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBytes(t *testing.T) {
	for value, expected := range map[uint64]string{
		0: "0 B", 1023: "1023 B", 1 << 10: "1.0 KiB", 1<<20 - 1: "1024 KiB",
		5.5 * (1 << 30): "5.5 GiB", 1 << 60: "1.0 EiB",
	} {
		assert.Equal(t, expected, FormatBytes(value), value)
	}

	for value, expected := range map[string]uint64{
		"42": 42, "42MB": 42000000, "42MiB": 44040192, "42 MB": 42000000,
		"42.5MB": 42500000, "42mi": 44040192, "1,005.03 MB": 1005030000,
		"12.5 EiB": 14411518807585587200,
	} {
		actual, err := ParseBytes(value)
		require.NoError(t, err, value)
		assert.Equal(t, expected, actual, value)
	}

	for _, value := range []string{"", "invalid", "84 JB", "16 EiB"} {
		_, err := ParseBytes(value)
		assert.Error(t, err, value)
	}
}
