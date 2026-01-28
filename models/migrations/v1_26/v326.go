// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

// AddOriginalUnixToAction adds original_unix column to action table
// for storing the original timestamp of content (e.g., commit author date).
// This allows the heatmap to display commits on their actual dates
// rather than the push date.
func AddOriginalUnixToAction(x *xorm.Engine) error {
	type Action struct {
		OriginalUnix int64 `xorm:"INDEX"`
	}

	return x.Sync(new(Action))
}
