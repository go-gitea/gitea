// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

func AddIssueWeight(x *xorm.Engine) error {
	type Issue struct {
		Weight int
	}
	return x.Sync(new(Issue))
}
