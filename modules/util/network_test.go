// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimPortFromIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 without port",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv4 with port",
			input:    "192.168.1.1:8080",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv6 without port",
			input:    "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 with brackets, without port",
			input:    "[2001:db8::1]",
			expected: "[2001:db8::1]",
		},
		{
			name:     "IPv6 with brackets and port",
			input:    "[2001:db8::1]:8080",
			expected: "[2001:db8::1]",
		},
		{
			name:     "localhost with port",
			input:    "localhost:8080",
			expected: "localhost",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Not an IP address",
			input:    "abc123",
			expected: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimPortFromIP(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
