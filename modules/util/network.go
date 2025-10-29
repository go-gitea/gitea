// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
)

// TrimPortFromIP removes the client port from an IP address
// Handles both IPv4 and IPv6 addresses with ports
func TrimPortFromIP(ip string) string {
	// Handle IPv6 with brackets: [IPv6]:port
	if strings.HasPrefix(ip, "[") {
		// If there's no port, return as is
		if !strings.Contains(ip, "]:") {
			return ip
		}
		// Remove the port part after ]:
		return strings.Split(ip, "]:")[0] + "]"
	}

	// Count colons to differentiate between IPv4 and IPv6
	colonCount := strings.Count(ip, ":")

	// Handle IPv4 with port (single colon)
	if colonCount == 1 {
		return strings.Split(ip, ":")[0]
	}

	return ip
}
