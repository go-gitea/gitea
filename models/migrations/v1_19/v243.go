// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"xorm.io/xorm"
)

func AddExclusiveLabel(x *xorm.Engine) error {
	type Label struct {
		Exclusive bool
	}

	return x.Sync(new(Label))
}
