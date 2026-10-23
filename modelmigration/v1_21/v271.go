// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"context"

	"codenest.dev/modelmigration/base"
	"codenest.dev/modules/timeutil"
)

func AddArchivedUnixColumInLabelTable(_ context.Context, x base.EngineMigration) error {
	type Label struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
	}
	return x.Sync(new(Label))
}
