// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import "fmt"

func maybeGroupSegment(gid int64) string {
	var seg string
	if gid > 0 {
		seg = fmt.Sprintf("group/%d/", gid)
	}
	return seg
}
