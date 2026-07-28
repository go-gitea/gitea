// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17

import "gitea.dev/modelmigration/base"

func CreateForeignReferenceTable(_ base.EngineMigration) error {
	return nil // This table was dropped in v1_19/v237.go
}
