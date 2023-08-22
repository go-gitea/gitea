// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"xorm.io/xorm"
)

func CreateForeignReferenceTable(_ *xorm.Engine) error {
	return nil // This table was dropped in v1_19/v237.go
}
