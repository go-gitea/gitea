// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddVersionToActionRunner(_ context.Context, x base.EngineMigration) error {
	type ActionRunner struct {
		Version string `xorm:"VARCHAR(64)"` // the version of the runner
	}

	return x.Sync(new(ActionRunner))
}
