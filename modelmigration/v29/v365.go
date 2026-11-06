// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddExternalAccountLink(_ctx context.Context, x base.EngineMigration) error {
	type ExternalAccountLink struct {
		ID          int64 `xorm:"pk autoincr"`
		ServiceType int   `xorm:"UNIQUE(s)"`
		ExternalID  int64 `xorm:"UNIQUE(s)"`
		UserID      int64 `xorm:"INDEX"`
	}

	return x.Sync(new(ExternalAccountLink))
}
