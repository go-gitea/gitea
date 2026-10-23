// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddVersionToActionRunTable(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		Version int `xorm:"version default 0"`
	}
	return x.Sync(new(ActionRun))
}
