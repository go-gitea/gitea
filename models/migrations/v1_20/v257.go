// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import "xorm.io/xorm"

func AddLabelsToActRunner(x *xorm.Engine) error {
	type ActRunner struct {
		Labels []string `xorm:"TEXT"`
	}

	// todo combine "agent labels" and "custom labels" to "labels"

	return x.Sync(new(ActRunner))
}
