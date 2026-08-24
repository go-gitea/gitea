// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	kiByte = 1 << 10
	miByte = 1 << 20
	giByte = 1 << 30
	eiByte = 1 << 60
)

func TestIBytes(t *testing.T) {
	for _, test := range []struct {
		value    uint64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{kiByte, "1.0 KiB"},
		{miByte - 1, "1024 KiB"},
		{5.5 * giByte, "5.5 GiB"},
		{eiByte, "1.0 EiB"},
	} {
		assert.Equal(t, test.expected, IBytes(test.value), "IBytes(%d)", test.value)
	}
}

func TestParseBytes(t *testing.T) {
	for _, test := range []struct {
		value    string
		expected uint64
	}{
		{"42", 42},
		{"42MB", 42000000},
		{"42MiB", 44040192},
		{"42 MB", 42000000},
		{"42.5MB", 42500000},
		{"42mi", 44040192},
		{"1,005.03 MB", 1005030000},
		{"12.5 EiB", 14411518807585587200},
	} {
		actual, err := ParseBytes(test.value)
		require.NoError(t, err, "ParseBytes(%q)", test.value)
		assert.Equal(t, test.expected, actual, "ParseBytes(%q)", test.value)
	}

	for _, value := range []string{"", "invalid", "84 JB", "16 EiB"} {
		_, err := ParseBytes(value)
		assert.Error(t, err, "ParseBytes(%q)", value)
	}
}
