// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeDomainName(t *testing.T) {
	for _, testCase := range []struct {
		input, expected string
	}{
		{"Example.COM", "example.com"},
		{"éxample.com", "xn--xample-9ua.com"},
	} {
		actual, err := normalizeDomainName(testCase.input)
		require.NoError(t, err)
		require.Equal(t, testCase.expected, actual)
	}

	for _, input := range []string{
		"localhost", "127.0.0.1", "example.com.", "example..com", "-example.com", "example-.com",
		"*.example.com", "example.com:443", "https://example.com", strings.Repeat("a", 64) + ".example",
	} {
		_, err := normalizeDomainName(input)
		require.ErrorIs(t, err, ErrInvalidAddress, input)
	}
}

func TestDNSNamespacesOverlapIgnoresCase(t *testing.T) {
	require.True(t, dnsNamespacesOverlap("Pages.Example.Test", "example.test"))
	require.True(t, dnsNamespacesOverlap("example.test", "Docs.Example.Test"))
	require.False(t, dnsNamespacesOverlap("notexample.test", "example.test"))
}
