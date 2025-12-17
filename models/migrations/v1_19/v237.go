// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"xorm.io/xorm"
)

func DropForeignReferenceTable(x *xorm.Engine) error {
	// Drop the table introduced in `v211`, it's considered badly designed and doesn't look like to be used.
	// See: https://github.com/go-gitea/gitea/issues/21086#issuecomment-1318217453
	type ForeignReference struct{}
	return x.DropTables(new(ForeignReference))
}
