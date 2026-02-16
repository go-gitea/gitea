// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agent

import "testing"

func TestIsEnrollmentRemoteAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		allowList  string
		expected   bool
	}{
		{name: "empty allow list allows all", remoteAddr: "203.0.113.2:1234", allowList: "", expected: true},
		{name: "single cidr allow", remoteAddr: "10.42.0.8:9999", allowList: "10.0.0.0/8", expected: true},
		{name: "single cidr deny", remoteAddr: "203.0.113.2:1234", allowList: "10.0.0.0/8", expected: false},
		{name: "builtin private allow", remoteAddr: "192.168.1.20:4567", allowList: "private", expected: true},
		{name: "builtin private deny public", remoteAddr: "198.51.100.10:4567", allowList: "private", expected: false},
		{name: "builtin loopback allow", remoteAddr: "127.0.0.1:4567", allowList: "loopback", expected: true},
		{name: "wildcard allow", remoteAddr: "198.51.100.10:4567", allowList: "*", expected: true},
		{name: "ipv6 cidr allow", remoteAddr: "[fd00::1234]:443", allowList: "fd00::/8", expected: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := IsEnrollmentRemoteAllowed(tt.remoteAddr, tt.allowList)
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}
