// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import "fmt"

func maybeGroupSegment(gid int64) string {
	if gid > 0 {
		return fmt.Sprintf("%d/", gid)
	}
	return ""
}

func maybeWebGroupSegment(gid int64) string {
	gs := maybeGroupSegment(gid)
	if gs != "" {
		return "group/" + gs
	}
	return ""
}
