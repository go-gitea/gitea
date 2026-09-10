// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "github.com/dustin/go-humanize"

// FormatByteSize formats a byte count as a user-friendly string, e.g. "1.5 MiB".
func FormatByteSize(size int64) string {
	return humanize.IBytes(uint64(size))
}

// ParseByteSize parses a byte size such as "100MB" or "1.5 GiB".
func ParseByteSize(value string) (uint64, error) {
	return humanize.ParseBytes(value)
}
