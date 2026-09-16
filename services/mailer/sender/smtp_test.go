// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import "testing"

func TestSanitizeEmailAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"abc@gitea.com", "abc@gitea.com", false},
		{"<abc@gitea.com>", "abc@gitea.com", false},
		{"ssss.com", "", true},
		{"<invalid-email>", "", true},
	}

	for _, tt := range tests {
		result, err := sanitizeEmailAddress(tt.input)
		if (err != nil) != tt.hasError {
			t.Errorf("sanitizeEmailAddress(%q) unexpected error status: got %v, want error: %v", tt.input, err != nil, tt.hasError)
			continue
		}
		if result != tt.expected {
			t.Errorf("sanitizeEmailAddress(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}
