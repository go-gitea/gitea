// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import "fmt"

func maybeGroupSegment(gid int64) string {
	if gid > 0 {
		return fmt.Sprintf("group/%d/", gid)
	}
	return ""
}
